// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Malloc profiling.
// Patterned after tcmalloc's algorithms; shorter code.

package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

// NOTE(rsc): Everything here could use cas if contention became an issue.
var proflock mutex

// All memory allocations are local and do not escape outside of the profiler.
// The profiler is forbidden from referring to garbage-collected memory.

const (
	// profile types
	memProfile bucketType = 1 + iota
	blockProfile
	mutexProfile

	// size of bucket hash table
	buckHashSize = 179999

	// max depth of stack to record in bucket
	maxStack = 32
)

type bucketType int

// A bucket holds per-call-stack profiling information.
// The representation is a bit sleazy, inherited from C.
// This struct defines the bucket header. It is followed in
// memory by the stack words and then the actual record
// data, either a memRecord or a blockRecord.
//
// Per-call-stack profiling information.
// Lookup by hashing call stack into a linked-list hash table.
//
// No heap pointers.
//
//go:notinheap
type bucket struct {
	next    *bucket
	allnext *bucket
	typ     bucketType // memBucket or blockBucket (includes mutexProfile)
	hash    uintptr
	size    uintptr
	nstk    uintptr
}

// A memRecord is the bucket data for a bucket of type memProfile,
// part of the memory profile.
type memRecord struct {
	// The following complex 3-stage scheme of stats accumulation
	// is required to obtain a consistent picture of mallocs and frees
	// for some point in time.
	// The problem is that mallocs come in real time, while frees
	// come only after a GC during concurrent sweeping. So if we would
	// naively count them, we would get a skew toward mallocs.
	//
	// Hence, we delay information to get consistent snapshots as
	// of mark termination. Allocations count toward the next mark
	// termination's snapshot, while sweep frees count toward the
	// previous mark termination's snapshot:
	//
	//              MT          MT          MT          MT
	//             .·|         .·|         .·|         .·|
	//          .·˙  |      .·˙  |      .·˙  |      .·˙  |
	//       .·˙     |   .·˙     |   .·˙     |   .·˙     |
	//    .·˙        |.·˙        |.·˙        |.·˙        |
	//
	//       alloc → ▲ ← free
	//               ┠┅┅┅┅┅┅┅┅┅┅┅P
	//       C+2     →    C+1    →  C
	//
	//                   alloc → ▲ ← free
	//                           ┠┅┅┅┅┅┅┅┅┅┅┅P
	//                   C+2     →    C+1    →  C
	//
	// Since we can't publish a consistent snapshot until all of
	// the sweep frees are accounted for, we wait until the next
	// mark termination ("MT" above) to publish the previous mark
	// termination's snapshot ("P" above). To do this, allocation
	// and free events are accounted to *future* heap profile
	// cycles ("C+n" above) and we only publish a cycle once all
	// of the events from that cycle must be done. Specifically:
	//
	// Mallocs are accounted to cycle C+2.
	// Explicit frees are accounted to cycle C+2.
	// GC frees (done during sweeping) are accounted to cycle C+1.
	//
	// After mark termination, we increment the global heap
	// profile cycle counter and accumulate the stats from cycle C
	// into the active profile.

	// active is the currently published profile. A profiling
	// cycle can be accumulated into active once its complete.
	active memRecordCycle

	// future records the profile events we're counting for cycles
	// that have not yet been published. This is ring buffer
	// indexed by the global heap profile cycle C and stores
	// cycles C, C+1, and C+2. Unlike active, these counts are
	// only for a single cycle; they are not cumulative across
	// cycles.
	//
	// We store cycle C here because there's a window between when
	// C becomes the active cycle and when we've flushed it to
	// active.
	future [3]memRecordCycle
}

// memRecordCycle
type memRecordCycle struct {
	allocs, frees           uintptr
	alloc_bytes, free_bytes uintptr
}

// add accumulates b into a. It does not zero b.
func (a *memRecordCycle) add(b *memRecordCycle) {
	a.allocs += b.allocs
	a.frees += b.frees
	a.alloc_bytes += b.alloc_bytes
	a.free_bytes += b.free_bytes
}

// A blockRecord is the bucket data for a bucket of type blockProfile,
// which is used in blocking and mutex profiles.
type blockRecord struct {
	count  int64
	cycles int64
}

var (
	mbuckets  *bucket // memory profile buckets
	bbuckets  *bucket // blocking profile buckets
	xbuckets  *bucket // mutex profile buckets
	buckhash  *[179999]*bucket
	bucketmem uintptr

	mProf struct {
		// All fields in mProf are protected by proflock.

		// cycle is the global heap profile cycle. This wraps
		// at mProfCycleWrap.
		cycle uint32
		// flushed indicates that future[cycle] in all buckets
		// has been flushed to the active profile.
		flushed bool
	}
)

const mProfCycleWrap = uint32(len(memRecord{}.future)) * (2 << 24)

// newBucket allocates a bucket with the given type and number of stack entries.
func newBucket(typ bucketType, nstk int) *bucket {
	size := unsafe.Sizeof(bucket{}) + uintptr(nstk)*unsafe.Sizeof(uintptr(0))
	switch typ {
	default:
		throw("invalid profile bucket type")
	case memProfile:
		size += unsafe.Sizeof(memRecord{})
	case blockProfile, mutexProfile:
		size += unsafe.Sizeof(blockRecord{})
	}

	b := (*bucket)(persistentalloc(size, 0, &memstats.buckhash_sys))
	bucketmem += size
	b.typ = typ
	b.nstk = uintptr(nstk)
	return b
}

// stk returns the slice in b holding the stack.
func (b *bucket) stk() []uintptr {
	stk := (*[maxStack]uintptr)(add(unsafe.Pointer(b), unsafe.Sizeof(*b)))
	return stk[:b.nstk:b.nstk]
}

// mp returns the memRecord associated with the memProfile bucket b.
func (b *bucket) mp() *memRecord {
	if b.typ != memProfile {
		throw("bad use of bucket.mp")
	}
	data := add(unsafe.Pointer(b), unsafe.Sizeof(*b)+b.nstk*unsafe.Sizeof(uintptr(0)))
	return (*memRecord)(data)
}

// bp returns the blockRecord associated with the blockProfile bucket b.
func (b *bucket) bp() *blockRecord {
	if b.typ != blockProfile && b.typ != mutexProfile {
		throw("bad use of bucket.bp")
	}
	data := add(unsafe.Pointer(b), unsafe.Sizeof(*b)+b.nstk*unsafe.Sizeof(uintptr(0)))
	return (*blockRecord)(data)
}

// Return the bucket for stk[0:nstk], allocating new bucket if needed.
func stkbucket(typ bucketType, size uintptr, stk []uintptr, alloc bool) *bucket {
	if buckhash == nil {
		buckhash = (*[buckHashSize]*bucket)(sysAlloc(unsafe.Sizeof(*buckhash), &memstats.buckhash_sys))
		if buckhash == nil {
			throw("runtime: cannot allocate memory")
		}
	}

	// Hash stack.
	var h uintptr
	for _, pc := range stk {
		h += pc
		h += h << 10
		h ^= h >> 6
	}
	// hash in size
	h += size
	h += h << 10
	h ^= h >> 6
	// finalize
	h += h << 3
	h ^= h >> 11

	i := int(h % buckHashSize)
	for b := buckhash[i]; b != nil; b = b.next {
		if b.typ == typ && b.hash == h && b.size == size && eqslice(b.stk(), stk) {
			return b
		}
	}

	if !alloc {
		return nil
	}

	// Create new bucket.
	b := newBucket(typ, len(stk))
	copy(b.stk(), stk)
	b.hash = h
	b.size = size
	b.next = buckhash[i]
	buckhash[i] = b
	if typ == memProfile {
		b.allnext = mbuckets
		mbuckets = b
	} else if typ == mutexProfile {
		b.allnext = xbuckets
		xbuckets = b
	} else {
		b.allnext = bbuckets
		bbuckets = b
	}
	return b
}

func eqslice(x, y []uintptr) bool {
	if len(x) != len(y) {
		return false
	}
	for i, xi := range x {
		if xi != y[i] {
			return false
		}
	}
	return true
}

// mProf_NextCycle publishes the next heap profile cycle and creates a
// fresh heap profile cycle. This operation is fast and can be done
// during STW. The caller must call mProf_Flush before calling
// mProf_NextCycle again.
//
// This is called by mark termination during STW so allocations and
// frees after the world is started again count towards a new heap
// profiling cycle.
func mProf_NextCycle() {
	lock(&proflock)
	// We explicitly wrap mProf.cycle rather than depending on
	// uint wraparound because the memRecord.future ring does not
	// itself wrap at a power of two.
	mProf.cycle = (mProf.cycle + 1) % mProfCycleWrap
	mProf.flushed = false
	unlock(&proflock)
}

// mProf_Flush flushes the events from the current heap profiling
// cycle into the active profile. After this it is safe to start a new
// heap profiling cycle with mProf_NextCycle.
//
// This is called by GC after mark termination starts the world. In
// contrast with mProf_NextCycle, this is somewhat expensive, but safe
// to do concurrently.
func mProf_Flush() {
	lock(&proflock)
	if !mProf.flushed {
		mProf_FlushLocked()
		mProf.flushed = true
	}
	unlock(&proflock)
}

func mProf_FlushLocked() {
	c := mProf.cycle
	for b := mbuckets; b != nil; b = b.allnext {
		mp := b.mp()

		// Flush cycle C into the published profile and clear
		// it for reuse.
		mpc := &mp.future[c%uint32(len(mp.future))]
		mp.active.add(mpc)
		*mpc = memRecordCycle{}
	}
}

// mProf_PostSweep records that all sweep frees for this GC cycle have
// completed. This has the effect of publishing the heap profile
// snapshot as of the last mark termination without advancing the heap
// profile cycle.
func mProf_PostSweep() {
	lock(&proflock)
	// Flush cycle C+1 to the active profile so everything as of
	// the last mark termination becomes visible. *Don't* advance
	// the cycle, since we're still accumulating allocs in cycle
	// C+2, which have to become C+1 in the next mark termination
	// and so on.
	c := mProf.cycle
	for b := mbuckets; b != nil; b = b.allnext {
		mp := b.mp()
		mpc := &mp.future[(c+1)%uint32(len(mp.future))]
		mp.active.add(mpc)
		*mpc = memRecordCycle{}
	}
	unlock(&proflock)
}

// Called by malloc to record a profiled block.
func mProf_Malloc(p unsafe.Pointer, size uintptr) {
	var stk [maxStack]uintptr
	nstk := callers(4, stk[:])
	lock(&proflock)
	b := stkbucket(memProfile, size, stk[:nstk], true)
	c := mProf.cycle
	mp := b.mp()
	mpc := &mp.future[(c+2)%uint32(len(mp.future))]
	mpc.allocs++
	mpc.alloc_bytes += size
	unlock(&proflock)

	// Setprofilebucket locks a bunch of other mutexes, so we call it outside of proflock.
	// This reduces potential contention and chances of deadlocks.
	// Since the object must be alive during call to mProf_Malloc,
	// it's fine to do this non-atomically.
	systemstack(func() {
		setprofilebucket(p, b)
	})
}

// Called when freeing a profiled block.
func mProf_Free(b *bucket, size uintptr) {
	lock(&proflock)
	c := mProf.cycle
	mp := b.mp()
	mpc := &mp.future[(c+1)%uint32(len(mp.future))]
	mpc.frees++
	mpc.free_bytes += size
	unlock(&proflock)
}

var blockprofilerate uint64 // in CPU ticks

// SetBlockProfileRate 控制阻塞 profile 中报告的 goroutine 阻塞事件的比例。
// profiler 的目标是以纳秒级阻塞的速率对平均一个阻塞事件进行采样。
// 要在 profile 中包含每个阻塞事件，传递 rate = 1。
// 要完全关闭 profiling，传递 rate <= 0。
func SetBlockProfileRate(rate int) {
	var r int64
	if rate <= 0 {
		r = 0 // disable profiling
	} else if rate == 1 {
		r = 1 // profile everything
	} else {
		// convert ns to cycles, use float64 to prevent overflow during multiplication
		r = int64(float64(rate) * float64(tickspersecond()) / (1000 * 1000 * 1000))
		if r == 0 {
			r = 1
		}
	}

	atomic.Store64(&blockprofilerate, uint64(r))
}

func blockevent(cycles int64, skip int) {
	if cycles <= 0 {
		cycles = 1
	}
	if blocksampled(cycles) {
		saveblockevent(cycles, skip+1, blockProfile)
	}
}

func blocksampled(cycles int64) bool {
	rate := int64(atomic.Load64(&blockprofilerate))
	if rate <= 0 || (rate > cycles && int64(fastrand())%rate > cycles) {
		return false
	}
	return true
}

func saveblockevent(cycles int64, skip int, which bucketType) {
	gp := getg()
	var nstk int
	var stk [maxStack]uintptr
	if gp.m.curg == nil || gp.m.curg == gp {
		nstk = callers(skip, stk[:])
	} else {
		nstk = gcallers(gp.m.curg, skip, stk[:])
	}
	lock(&proflock)
	b := stkbucket(which, 0, stk[:nstk], true)
	b.bp().count++
	b.bp().cycles += cycles
	unlock(&proflock)
}

var mutexprofilerate uint64 // fraction sampled

// SetMutexProfileFraction 控制 mutex profile 中报告的互斥锁争用事件的比例。
// 平均 1/rate 事件被报告。返回以前的速率 rate。
//
// 要完全关闭 profiling，传递 rate 为 0。
// 要只读取当前速率，通过 rate < 0。(对于 n>1，采样细节可能会改变。)
func SetMutexProfileFraction(rate int) int {
	if rate < 0 {
		return int(mutexprofilerate)
	}
	old := mutexprofilerate
	atomic.Store64(&mutexprofilerate, uint64(rate))
	return int(old)
}

//go:linkname mutexevent sync.event
func mutexevent(cycles int64, skip int) {
	if cycles < 0 {
		cycles = 0
	}
	rate := int64(atomic.Load64(&mutexprofilerate))
	// TODO(pjw): measure impact of always calling fastrand vs using something
	// like malloc.go:nextSample()
	if rate > 0 && int64(fastrand())%rate == 0 {
		saveblockevent(cycles, skip+1, mutexProfile)
	}
}

// Go interface to profile data.

// StackRecord 描述单个执行堆栈。
type StackRecord struct {
	Stack0 [32]uintptr // stack trace for this record; ends at first 0 entry
}

// Stack 返回与记录相关联的堆栈跟踪，前缀为 r.Stack0。
func (r *StackRecord) Stack() []uintptr {
	for i, v := range r.Stack0 {
		if v == 0 {
			return r.Stack0[0:i]
		}
	}
	return r.Stack0[0:]
}

// MemProfileRate 控制 memory profile 中记录和报告的内存分配比例。
// profiler 的目标是为每个分配的 MemProfileRate bytes 抽样一个分配平均值。
//
// 要在 profile 中包含每个已分配的块，请将 MemProfileRate 设置为 1。
// 若要完全关闭分析，设置 MemProfileRate 为 0。
//
// 处理 memory profiles 的工具假设 profile 速率在程序的整个生命周期内是恒定的，并且等于当前值。
// 改变内存分析速率的程序应该只改变一次，在程序执行时尽早改变(例如，在 main 的开始)。
var MemProfileRate int = 512 * 1024

// MemProfileRecord 描述了由特定调用序列(堆栈跟踪)分配的活动对象。
type MemProfileRecord struct {
	AllocBytes, FreeBytes     int64       // number of bytes allocated, freed
	AllocObjects, FreeObjects int64       // number of objects allocated, freed
	Stack0                    [32]uintptr // stack trace for this record; ends at first 0 entry
}

// InUseBytes 返回正在使用的字节数(AllocBytes - FreeBytes)。
func (r *MemProfileRecord) InUseBytes() int64 { return r.AllocBytes - r.FreeBytes }

// InUseObjects 返回正在使用的对象数(AllocObjects - FreeObjects)。
func (r *MemProfileRecord) InUseObjects() int64 {
	return r.AllocObjects - r.FreeObjects
}

// Stack 返回与记录相关联的堆栈跟踪，前缀为 r.Stack0。
func (r *MemProfileRecord) Stack() []uintptr {
	for i, v := range r.Stack0 {
		if v == 0 {
			return r.Stack0[0:i]
		}
	}
	return r.Stack0[0:]
}

// MemProfile 返回每个分配点分配和释放的内存 profile。
//
// MemProfile 返回 n，即当前 memory profile 中的记录数。
// 如果 len(p) >= n，MemProfile 将该 profile 复制到 p 中，并返回 n，true。
// 如果 len(p) < n，MemProfile 不会改变 p，并返回 n, false。
//
// 如果 inuseZero 为 true，profile 包括分配记录，其中 r.AllocBytes > 0，但 r.AllocBytes == r.FreeBytes。
// 这些是内存被分配的点，但是它已经被释放回运行时。
// 
// 返回的 profile 可能长达两个垃圾收集周期。
// 这是为了避免 profile 偏向分配；因为分配是实时发生的，但是释放被延迟，直到垃圾收集器执行清理，
// 这个 profile 只考虑有机会被垃圾收集器释放的分配。
//
// 大多数客户端应该使用 runtime/pprof 包或 testing 包的 -test.memprofile 标志，而不是直接调用memprofile。
func MemProfile(p []MemProfileRecord, inuseZero bool) (n int, ok bool) {
	lock(&proflock)
	// If we're between mProf_NextCycle and mProf_Flush, take care
	// of flushing to the active profile so we only have to look
	// at the active profile below.
	mProf_FlushLocked()
	clear := true
	for b := mbuckets; b != nil; b = b.allnext {
		mp := b.mp()
		if inuseZero || mp.active.alloc_bytes != mp.active.free_bytes {
			n++
		}
		if mp.active.allocs != 0 || mp.active.frees != 0 {
			clear = false
		}
	}
	if clear {
		// Absolutely no data, suggesting that a garbage collection
		// has not yet happened. In order to allow profiling when
		// garbage collection is disabled from the beginning of execution,
		// accumulate all of the cycles, and recount buckets.
		n = 0
		for b := mbuckets; b != nil; b = b.allnext {
			mp := b.mp()
			for c := range mp.future {
				mp.active.add(&mp.future[c])
				mp.future[c] = memRecordCycle{}
			}
			if inuseZero || mp.active.alloc_bytes != mp.active.free_bytes {
				n++
			}
		}
	}
	if n <= len(p) {
		ok = true
		idx := 0
		for b := mbuckets; b != nil; b = b.allnext {
			mp := b.mp()
			if inuseZero || mp.active.alloc_bytes != mp.active.free_bytes {
				record(&p[idx], b)
				idx++
			}
		}
	}
	unlock(&proflock)
	return
}

// Write b's data to r.
func record(r *MemProfileRecord, b *bucket) {
	mp := b.mp()
	r.AllocBytes = int64(mp.active.alloc_bytes)
	r.FreeBytes = int64(mp.active.free_bytes)
	r.AllocObjects = int64(mp.active.allocs)
	r.FreeObjects = int64(mp.active.frees)
	if raceenabled {
		racewriterangepc(unsafe.Pointer(&r.Stack0[0]), unsafe.Sizeof(r.Stack0), getcallerpc(), funcPC(MemProfile))
	}
	if msanenabled {
		msanwrite(unsafe.Pointer(&r.Stack0[0]), unsafe.Sizeof(r.Stack0))
	}
	copy(r.Stack0[:], b.stk())
	for i := int(b.nstk); i < len(r.Stack0); i++ {
		r.Stack0[i] = 0
	}
}

func iterate_memprof(fn func(*bucket, uintptr, *uintptr, uintptr, uintptr, uintptr)) {
	lock(&proflock)
	for b := mbuckets; b != nil; b = b.allnext {
		mp := b.mp()
		fn(b, b.nstk, &b.stk()[0], b.size, mp.active.allocs, mp.active.frees)
	}
	unlock(&proflock)
}

// BlockProfileRecord 记录描述了源自特定调用序列(堆栈跟踪)的阻塞事件。
type BlockProfileRecord struct {
	Count  int64
	Cycles int64
	StackRecord
}

// BlockProfile 返回 n，即当前阻塞 profile 的记录数。
// 如果 len(p) >= n，BlockProfile 复制 profile 到 p 并返回 n, true。
// 如果 len(p) < n，BlockProfile 不会改变 p 并返回 n, false。 
//
// 大多数客户端应该使用 runtime/pprof 包或
// testing 包的 -test.blockprofile 标志，而不是直接调用 BlockProfile
func BlockProfile(p []BlockProfileRecord) (n int, ok bool) {
	lock(&proflock)
	for b := bbuckets; b != nil; b = b.allnext {
		n++
	}
	if n <= len(p) {
		ok = true
		for b := bbuckets; b != nil; b = b.allnext {
			bp := b.bp()
			r := &p[0]
			r.Count = bp.count
			r.Cycles = bp.cycles
			if raceenabled {
				racewriterangepc(unsafe.Pointer(&r.Stack0[0]), unsafe.Sizeof(r.Stack0), getcallerpc(), funcPC(BlockProfile))
			}
			if msanenabled {
				msanwrite(unsafe.Pointer(&r.Stack0[0]), unsafe.Sizeof(r.Stack0))
			}
			i := copy(r.Stack0[:], b.stk())
			for ; i < len(r.Stack0); i++ {
				r.Stack0[i] = 0
			}
			p = p[1:]
		}
	}
	unlock(&proflock)
	return
}

// MutexProfile 返回 n，当前 mutex profile 中的记录数。
// 如果 len(p) >= n，MutexProfile 将该配置文件复制到p，并返回n，true。
// 否则，MutexProfile 不改变p，返回n，false。
//
// 大多数客户端应该使用 runtime/pprof 包，而不是直接调用 MutexProfile。
func MutexProfile(p []BlockProfileRecord) (n int, ok bool) {
	lock(&proflock)
	for b := xbuckets; b != nil; b = b.allnext {
		n++
	}
	if n <= len(p) {
		ok = true
		for b := xbuckets; b != nil; b = b.allnext {
			bp := b.bp()
			r := &p[0]
			r.Count = int64(bp.count)
			r.Cycles = bp.cycles
			i := copy(r.Stack0[:], b.stk())
			for ; i < len(r.Stack0); i++ {
				r.Stack0[i] = 0
			}
			p = p[1:]
		}
	}
	unlock(&proflock)
	return
}

// ThreadCreateProfile returns n, the number of records in the thread creation profile.
// ThreadCreateProfile 返回 n，表示 thread creation profile 记录的数量。
// 如果 len(p) >= n，ThreadCreateProfile 复制 profile 到 p 并返回 n, true。
// 如果 len(p) < n，ThreadCreateProfile 不会改变 p 并返回 n, false。
//
// 大多数客户端应该使用 runtime/pprof 包而不是直接调用 ThreadCreateProfile。
func ThreadCreateProfile(p []StackRecord) (n int, ok bool) {
	first := (*m)(atomic.Loadp(unsafe.Pointer(&allm)))
	for mp := first; mp != nil; mp = mp.alllink {
		n++
	}
	if n <= len(p) {
		ok = true
		i := 0
		for mp := first; mp != nil; mp = mp.alllink {
			p[i].Stack0 = mp.createstack
			i++
		}
	}
	return
}

//go:linkname runtime_goroutineProfileWithLabels runtime/pprof.runtime_goroutineProfileWithLabels
func runtime_goroutineProfileWithLabels(p []StackRecord, labels []unsafe.Pointer) (n int, ok bool) {
	return goroutineProfileWithLabels(p, labels)
}

// labels may be nil. If labels is non-nil, it must have the same length as p.
func goroutineProfileWithLabels(p []StackRecord, labels []unsafe.Pointer) (n int, ok bool) {
	if labels != nil && len(labels) != len(p) {
		labels = nil
	}
	gp := getg()

	isOK := func(gp1 *g) bool {
		// Checking isSystemGoroutine here makes GoroutineProfile
		// consistent with both NumGoroutine and Stack.
		return gp1 != gp && readgstatus(gp1) != _Gdead && !isSystemGoroutine(gp1, false)
	}

	stopTheWorld("profile")

	n = 1
	for _, gp1 := range allgs {
		if isOK(gp1) {
			n++
		}
	}

	if n <= len(p) {
		ok = true
		r, lbl := p, labels

		// Save current goroutine.
		sp := getcallersp()
		pc := getcallerpc()
		systemstack(func() {
			saveg(pc, sp, gp, &r[0])
		})
		r = r[1:]

		// If we have a place to put our goroutine labelmap, insert it there.
		if labels != nil {
			lbl[0] = gp.labels
			lbl = lbl[1:]
		}

		// Save other goroutines.
		for _, gp1 := range allgs {
			if isOK(gp1) {
				if len(r) == 0 {
					// Should be impossible, but better to return a
					// truncated profile than to crash the entire process.
					break
				}
				saveg(^uintptr(0), ^uintptr(0), gp1, &r[0])
				if labels != nil {
					lbl[0] = gp1.labels
					lbl = lbl[1:]
				}
				r = r[1:]
			}
		}
	}

	startTheWorld()
	return n, ok
}

// GoroutineProfile 返回 n，活动 goroutine stack profile 的记录数。
// 如果 len(p) >= n，GoroutineProfile 将 profile 复制到 p 并返回 n，true。
// 如果 len(p) < n，GoroutineProfile 不改变 p，返回 n，false。
//
// 大多数客户端应该使用 runtime/pprof 包，而不是直接调用 GoroutineProfile。
func GoroutineProfile(p []StackRecord) (n int, ok bool) {

	return goroutineProfileWithLabels(p, nil)
}

func saveg(pc, sp uintptr, gp *g, r *StackRecord) {
	n := gentraceback(pc, sp, 0, gp, 0, &r.Stack0[0], len(r.Stack0), nil, nil, 0)
	if n < len(r.Stack0) {
		r.Stack0[n] = 0
	}
}

// Stack 将调用 goroutine 的堆栈跟踪格式化为 buf，
// 并返回写入 buf 的字节数。如果 all 为 true，
// Stack 将所有其他 goroutine 的堆栈跟踪格式化为当前 goroutine 跟踪之后的buf。
func Stack(buf []byte, all bool) int {
	if all {
		stopTheWorld("stack trace")
	}

	n := 0
	if len(buf) > 0 {
		gp := getg()
		sp := getcallersp()
		pc := getcallerpc()
		systemstack(func() {
			g0 := getg()
			// Force traceback=1 to override GOTRACEBACK setting,
			// so that Stack's results are consistent.
			// GOTRACEBACK is only about crash dumps.
			g0.m.traceback = 1
			g0.writebuf = buf[0:0:len(buf)]
			goroutineheader(gp)
			traceback(pc, sp, 0, gp)
			if all {
				tracebackothers(gp)
			}
			g0.m.traceback = 0
			n = len(g0.writebuf)
			g0.writebuf = nil
		})
	}

	if all {
		startTheWorld()
	}
	return n
}

// Tracing of alloc/free/gc.

var tracelock mutex

func tracealloc(p unsafe.Pointer, size uintptr, typ *_type) {
	lock(&tracelock)
	gp := getg()
	gp.m.traceback = 2
	if typ == nil {
		print("tracealloc(", p, ", ", hex(size), ")\n")
	} else {
		print("tracealloc(", p, ", ", hex(size), ", ", typ.string(), ")\n")
	}
	if gp.m.curg == nil || gp == gp.m.curg {
		goroutineheader(gp)
		pc := getcallerpc()
		sp := getcallersp()
		systemstack(func() {
			traceback(pc, sp, 0, gp)
		})
	} else {
		goroutineheader(gp.m.curg)
		traceback(^uintptr(0), ^uintptr(0), 0, gp.m.curg)
	}
	print("\n")
	gp.m.traceback = 0
	unlock(&tracelock)
}

func tracefree(p unsafe.Pointer, size uintptr) {
	lock(&tracelock)
	gp := getg()
	gp.m.traceback = 2
	print("tracefree(", p, ", ", hex(size), ")\n")
	goroutineheader(gp)
	pc := getcallerpc()
	sp := getcallersp()
	systemstack(func() {
		traceback(pc, sp, 0, gp)
	})
	print("\n")
	gp.m.traceback = 0
	unlock(&tracelock)
}

func tracegc() {
	lock(&tracelock)
	gp := getg()
	gp.m.traceback = 2
	print("tracegc()\n")
	// running on m->g0 stack; show all non-g0 goroutines
	tracebackothers(gp)
	print("end tracegc\n")
	print("\n")
	gp.m.traceback = 0
	unlock(&tracelock)
}
