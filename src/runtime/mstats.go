// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Memory statistics

package runtime

import (
	"runtime/internal/atomic"
	"runtime/internal/sys"
	"unsafe"
)

// Statistics.
// If you edit this structure, also edit type MemStats below.
// Their layouts must match exactly.
//
// For detailed descriptions see the documentation for MemStats.
// Fields that differ from MemStats are further documented here.
//
// Many of these fields are updated on the fly, while others are only
// updated when updatememstats is called.
type mstats struct {
	// General statistics.
	alloc       uint64 // bytes allocated and not yet freed
	total_alloc uint64 // bytes allocated (even if freed)
	sys         uint64 // bytes obtained from system (should be sum of xxx_sys below, no locking, approximate)
	nlookup     uint64 // number of pointer lookups (unused)
	nmalloc     uint64 // number of mallocs
	nfree       uint64 // number of frees

	// Statistics about malloc heap.
	// Updated atomically, or with the world stopped.
	//
	// Like MemStats, heap_sys and heap_inuse do not count memory
	// in manually-managed spans.
	heap_alloc    uint64 // bytes allocated and not yet freed (same as alloc above)
	heap_sys      uint64 // virtual address space obtained from system for GC'd heap
	heap_idle     uint64 // bytes in idle spans
	heap_inuse    uint64 // bytes in mSpanInUse spans
	heap_released uint64 // bytes released to the os

	// heap_objects is not used by the runtime directly and instead
	// computed on the fly by updatememstats.
	heap_objects uint64 // total number of allocated objects

	// Statistics about allocation of low-level fixed-size structures.
	// Protected by FixAlloc locks.
	stacks_inuse uint64 // bytes in manually-managed stack spans; updated atomically or during STW
	stacks_sys   uint64 // only counts newosproc0 stack in mstats; differs from MemStats.StackSys
	mspan_inuse  uint64 // mspan structures
	mspan_sys    uint64
	mcache_inuse uint64 // mcache structures
	mcache_sys   uint64
	buckhash_sys uint64 // profiling bucket hash table
	gc_sys       uint64 // updated atomically or during STW
	other_sys    uint64 // updated atomically or during STW

	// Statistics about garbage collector.
	// Protected by mheap or stopping the world during GC.
	next_gc         uint64 // goal heap_live for when next GC ends; ^0 if disabled
	last_gc_unix    uint64 // last gc (in unix time)
	pause_total_ns  uint64
	pause_ns        [256]uint64 // circular buffer of recent gc pause lengths
	pause_end       [256]uint64 // circular buffer of recent gc end times (nanoseconds since 1970)
	numgc           uint32
	numforcedgc     uint32  // number of user-forced GCs
	gc_cpu_fraction float64 // fraction of CPU time used by GC
	enablegc        bool
	debuggc         bool

	// Statistics about allocation size classes.

	by_size [_NumSizeClasses]struct {
		size    uint32
		nmalloc uint64
		nfree   uint64
	}

	// Statistics below here are not exported to MemStats directly.

	last_gc_nanotime uint64 // last gc (monotonic time)
	tinyallocs       uint64 // number of tiny allocations that didn't cause actual allocation; not exported to go directly
	last_next_gc     uint64 // next_gc for the previous GC
	last_heap_inuse  uint64 // heap_inuse at mark termination of the previous GC

	// triggerRatio is the heap growth ratio that triggers marking.
	//
	// E.g., if this is 0.6, then GC should start when the live
	// heap has reached 1.6 times the heap size marked by the
	// previous cycle. This should be ≤ GOGC/100 so the trigger
	// heap size is less than the goal heap size. This is set
	// during mark termination for the next cycle's trigger.
	triggerRatio float64

	// gc_trigger is the heap size that triggers marking.
	//
	// When heap_live ≥ gc_trigger, the mark phase will start.
	// This is also the heap size by which proportional sweeping
	// must be complete.
	//
	// This is computed from triggerRatio during mark termination
	// for the next cycle's trigger.
	gc_trigger uint64

	// heap_live is the number of bytes considered live by the GC.
	// That is: retained by the most recent GC plus allocated
	// since then. heap_live <= heap_alloc, since heap_alloc
	// includes unmarked objects that have not yet been swept (and
	// hence goes up as we allocate and down as we sweep) while
	// heap_live excludes these objects (and hence only goes up
	// between GCs).
	//
	// This is updated atomically without locking. To reduce
	// contention, this is updated only when obtaining a span from
	// an mcentral and at this point it counts all of the
	// unallocated slots in that span (which will be allocated
	// before that mcache obtains another span from that
	// mcentral). Hence, it slightly overestimates the "true" live
	// heap size. It's better to overestimate than to
	// underestimate because 1) this triggers the GC earlier than
	// necessary rather than potentially too late and 2) this
	// leads to a conservative GC rate rather than a GC rate that
	// is potentially too low.
	//
	// Reads should likewise be atomic (or during STW).
	//
	// Whenever this is updated, call traceHeapAlloc() and
	// gcController.revise().
	heap_live uint64

	// heap_scan is the number of bytes of "scannable" heap. This
	// is the live heap (as counted by heap_live), but omitting
	// no-scan objects and no-scan tails of objects.
	//
	// Whenever this is updated, call gcController.revise().
	heap_scan uint64

	// heap_marked is the number of bytes marked by the previous
	// GC. After mark termination, heap_live == heap_marked, but
	// unlike heap_live, heap_marked does not change until the
	// next mark termination.
	heap_marked uint64
}

var memstats mstats

// MemStats 记录内存分配器的统计信息。
type MemStats struct {
	// General statistics.

	// Alloc 为分配的堆对象的字节数
	//
	// 这和 HeapAlloc 相同 (看下面)。
	Alloc uint64

	// TotalAlloc 为堆对象分配的累积字节数
	// 
	// TotalAlloc 随着堆对象的分配而增加，但与 Alloc 和 HeapAlloc 不同，
	// 当对象被释放时，TotalAlloc 不会减少。
	TotalAlloc uint64

	// Sys 是从操作系统获得的总内存字节数。
	// 
	// Sys 是下面 XSs 字段的总和。
	// Sys 度量 Go 运行时为堆、栈和其他内部数据结构保留的虚拟地址空间。
	// 很可能在任何给定时刻，并非所有的虚拟地址空间都由物理内存支持，尽管通常在某个时刻都是如此。
	Sys uint64

	// Lookups 是运行时执行的指针查找次数。
	//
	// 这主要用于调试运行时内部。
	Lookups uint64

	// Mallocs 是分配的堆对象的累积计数。
	// 活动对象的数量是 Mallocs - Frees。
	Mallocs uint64

	// Frees 是释放的堆对象的累积计数。
	Frees uint64

	// Heap memory statistics.
	//
	// Interpreting the heap statistics requires some knowledge of
	// how Go organizes memory. Go divides the virtual address
	// space of the heap into "spans", which are contiguous
	// regions of memory 8K or larger. A span may be in one of
	// three states:
	//
	// An "idle" span contains no objects or other data. The
	// physical memory backing an idle span can be released back
	// to the OS (but the virtual address space never is), or it
	// can be converted into an "in use" or "stack" span.
	//
	// An "in use" span contains at least one heap object and may
	// have free space available to allocate more heap objects.
	//
	// A "stack" span is used for goroutine stacks. Stack spans
	// are not considered part of the heap. A span can change
	// between heap and stack memory; it is never used for both
	// simultaneously.

	// HeapAlloc 是分配的堆对象字节数。
	//
	// "Allocated" 堆对象包括所有可达的对象，以及垃圾收集器尚未释放的无法访问的对象。
	// 具体来说，HeapAlloc 随着堆对象的分配而增加，随着堆的清理和不可达对象的释放而减少。
	// 清扫在垃圾收集周期之间以增量方式进行，因此这两个过程同时发生，
	// 因此 HeapAlloc 倾向于平滑变化(与典型的 stop-the-world 垃圾收集器的锯齿形成对比)
	HeapAlloc uint64

	// HeapSys 是从操作系统获得的堆内存字节数。
	//
	// HeapSys 度量为堆保留的虚拟地址空间量。这包括已保留但尚未使用的虚拟地址空间，
	// 它不消耗物理内存，但往往很小，以及物理内存在未使用后已返回给操作系统的虚拟地址空间(后者的度量见 HeapReleased)。
	HeapSys uint64

	// HeapIdle 是空闲(未使用)跨度中的字节数。
	//
	// 空闲跨度中没有对象。这些跨度可以(并且可能已经)返回给操作系统，
	// 或者它们可以被重新用于堆分配，或者它们可以被重新用作栈内存。
	//
	// HeapIdle 减去 HeapReleased 估计可以返回给操作系统的内存量，
	// 但由运行时保留，因此它可以增加堆，而无需向操作系统请求更多内存。
	// 如果该差异明显大于堆大小，则表明最近活动堆大小出现了短暂的峰值。
	HeapIdle uint64

	// HeapInuse 是正在使用的内存跨度的字节数。
	//
	// 正在使用的内存单元至少包含一个对象。这些跨度只能用于大小大致相同的其他对象。
	//
	// HeapInuse 减去 HeapAlloc 估计专用于特定大小类但当前未被使用的内存量。
	// 这是碎片的上限，但一般来说，这种内存可以有效地重用。
	HeapInuse uint64

	// HeapReleased 是返回给操作系统的物理内存字节数。
	//
	// 这将从返回给操作系统且尚未为堆重新获取的空闲跨度中计算堆内存。
	HeapReleased uint64
 
	// HeapObjects 是分配的堆对象的数量。
	//
	// 与 HeapAlloc 一样，这种情况随着对象的分配而增加，随着堆的清理和不可访问对象的释放而减少。
	HeapObjects uint64

	// Stack memory statistics.
	//
	// Stacks are not considered part of the heap, but the runtime
	// can reuse a span of heap memory for stack memory, and
	// vice-versa.

	// StackInuse 是栈跨度中的字节。
	//
	// 正在使用的栈跨度中至少有一个栈。这些跨度只能用于相同大小的其他栈。
	//
	// 没有 StackIdle，因为未使用的堆栈跨度返回到堆中(因此计入堆)。
	StackInuse uint64

	// StackSys 是从 OS 获得的栈内存的字节数。
	//
	// StackSys 是 StackInuse 加上直接从 OS 获得的用于 OS 线程栈的任何内存(应该是非常小的)。
	StackSys uint64

	// Off-heap memory statistics.
	//
	// The following statistics measure runtime-internal
	// structures that are not allocated from heap memory (usually
	// because they are part of implementing the heap). Unlike
	// heap or stack memory, any memory allocated to these
	// structures is dedicated to these structures.
	//
	// These are primarily useful for debugging runtime memory
	// overheads.

	// MSpanInuse 是分配的 mspan 结构的字节数。
	MSpanInuse uint64

	// MSpanSys 是从 OS 获得的用于 mspan 结构的内存字节数。
	MSpanSys uint64

	// MCacheInuse 是分配的 mcache 结构的字节数。
	MCacheInuse uint64

	// MCacheSys 是从操作系统获得的用于 mcache 结构的内存字节数。
	MCacheSys uint64

	// BuckHashSys 是分析桶哈希表中的字节内存。
	BuckHashSys uint64

	// GCSys 是垃圾收集元数据中的内存字节数。
	GCSys uint64

	// OtherSys 是杂项堆外运行时分配中的内存字节数。
	OtherSys uint64

	// Garbage collector statistics.

	// NextGC 是下一个 GC 周期的目标堆大小。
	// 
	// 垃圾收集器的目标是保持 HeapAlloc ≤ NextGC。
	// 在每个垃圾收集周期结束时，将根据可到达的数据量和 GOGC 值计算下一个周期的目标。
	NextGC uint64

	// LastGC 是自1970年(UNIX纪元)以来最后一次垃圾收集完成的时间，以纳秒为单位。
	LastGC uint64

	// PauseTotalNs 是自程序启动以来 GC stop-the-world 暂停的累积纳秒数。
	//
	// 在 stop-the-world 期间，所有 goroutines 都会暂停，只有垃圾收集器可以运行
	PauseTotalNs uint64

	// PauseNs 是最近 GC stop-the-world 暂停时间的循环缓冲区，以纳秒为单位。
	//
	// 最近一次暂停是在 PauseNs[(NumGC+255)%256]。
	// 一般来说，PauseNs[N%256] 记录最近 N%256 次 GC 循环中暂停的时间。
	// 每个 GC 循环可能有多个暂停；这是一个周期内所有暂停的总和。
	PauseNs [256]uint64

	// PauseEnd 是最近 GC 暂停结束时间的循环缓冲区，从1970年(UNIX纪元)开始以纳秒为单位。
	//
	// 该缓冲区的填充方式与 PauseNs 相同。每个 GC 循环可能有多个暂停；
	// 这记录了循环中最后一次暂停的结束。
	PauseEnd [256]uint64

	// NumGC 是已完成的 GC 周期数。
	NumGC uint32

	// NumForcedGC 是调用 GC 函数的应用程序强制的 GC 周期数。
	NumForcedGC uint32

	// GCCPUFraction 是自程序启动以来，GC 使用的该程序的可用 CPU 时间的一部分。
	//
	// GCCPUFraction 表示为 0 到 1 之间的数字，
	// 其中 0 表示 GC 没有消耗该程序的任何 CPU。一个程序的可用 CPU 时间被定义为自程序启动以来 GOMAXPROCS 的积分。
	// 也就是说，如果 GOMAXPROCS 是 2，一个程序已经运行了 10 秒，那么它的“可用CPU”就是20秒。
	// GCCPUFraction 不包括用于写屏障活动的 CPU 时间。
	//
	// 这个和 GODEBUG=gctrace=1 报的 CPU 分数一样。
	GCCPUFraction float64

	// EnableGC 表示启用了 GC。它总是启用的，即使 GOGC=off。
	EnableGC bool

	// DebugGC 当前未使用。
	DebugGC bool

	// BySize 报告每一个 sized-classes 分配的统计信息。
	//
	// BySize[N] 给出大小为 S 的分配的统计数据，
	// 其中 BySize[N-1]。Size < S ≤ BySize[N].Size。
	//
	// 这不会报告大于 BySize[60] 大小的分配。
	BySize [61]struct {
		// Size is the maximum byte size of an object in this
		// size class.
		Size uint32

		// Mallocs is the cumulative count of heap objects
		// allocated in this size class. The cumulative bytes
		// of allocation is Size*Mallocs. The number of live
		// objects in this size class is Mallocs - Frees.
		Mallocs uint64

		// Frees is the cumulative count of heap objects freed
		// in this size class.
		Frees uint64
	}
}

// Size of the trailing by_size array differs between mstats and MemStats,
// and all data after by_size is local to runtime, not exported.
// NumSizeClasses was changed, but we cannot change MemStats because of backward compatibility.
// sizeof_C_MStats is the size of the prefix of mstats that
// corresponds to MemStats. It should match Sizeof(MemStats{}).
var sizeof_C_MStats = unsafe.Offsetof(memstats.by_size) + 61*unsafe.Sizeof(memstats.by_size[0])

func init() {
	var memStats MemStats
	if sizeof_C_MStats != unsafe.Sizeof(memStats) {
		println(sizeof_C_MStats, unsafe.Sizeof(memStats))
		throw("MStats vs MemStatsType size mismatch")
	}

	if unsafe.Offsetof(memstats.heap_live)%8 != 0 {
		println(unsafe.Offsetof(memstats.heap_live))
		throw("memstats.heap_live not aligned to 8 bytes")
	}
}

// ReadMemStats 用内存分配器统计信息填充 m。
// 
// 返回的内存分配器统计信息在调用 ReadMemStats 时是最新的。
// 这与 heap profile 形成对比，heap profile 是最近完成的垃圾收集周期的快照。
func ReadMemStats(m *MemStats) {
	stopTheWorld("read mem stats")

	systemstack(func() {
		readmemstats_m(m)
	})

	startTheWorld()
}

func readmemstats_m(stats *MemStats) {
	updatememstats()

	// The size of the trailing by_size array differs between
	// mstats and MemStats. NumSizeClasses was changed, but we
	// cannot change MemStats because of backward compatibility.
	memmove(unsafe.Pointer(stats), unsafe.Pointer(&memstats), sizeof_C_MStats)

	// memstats.stacks_sys is only memory mapped directly for OS stacks.
	// Add in heap-allocated stack memory for user consumption.
	stats.StackSys += stats.StackInuse
}

//go:linkname readGCStats runtime/debug.readGCStats
func readGCStats(pauses *[]uint64) {
	systemstack(func() {
		readGCStats_m(pauses)
	})
}

// readGCStats_m must be called on the system stack because it acquires the heap
// lock. See mheap for details.
//go:systemstack
func readGCStats_m(pauses *[]uint64) {
	p := *pauses
	// Calling code in runtime/debug should make the slice large enough.
	if cap(p) < len(memstats.pause_ns)+3 {
		throw("short slice passed to readGCStats")
	}

	// Pass back: pauses, pause ends, last gc (absolute time), number of gc, total pause ns.
	lock(&mheap_.lock)

	n := memstats.numgc
	if n > uint32(len(memstats.pause_ns)) {
		n = uint32(len(memstats.pause_ns))
	}

	// The pause buffer is circular. The most recent pause is at
	// pause_ns[(numgc-1)%len(pause_ns)], and then backward
	// from there to go back farther in time. We deliver the times
	// most recent first (in p[0]).
	p = p[:cap(p)]
	for i := uint32(0); i < n; i++ {
		j := (memstats.numgc - 1 - i) % uint32(len(memstats.pause_ns))
		p[i] = memstats.pause_ns[j]
		p[n+i] = memstats.pause_end[j]
	}

	p[n+n] = memstats.last_gc_unix
	p[n+n+1] = uint64(memstats.numgc)
	p[n+n+2] = memstats.pause_total_ns
	unlock(&mheap_.lock)
	*pauses = p[:n+n+3]
}

//go:nowritebarrier
func updatememstats() {
	// Flush mcaches to mcentral before doing anything else.
	//
	// Flushing to the mcentral may in general cause stats to
	// change as mcentral data structures are manipulated.
	systemstack(flushallmcaches)

	memstats.mcache_inuse = uint64(mheap_.cachealloc.inuse)
	memstats.mspan_inuse = uint64(mheap_.spanalloc.inuse)
	memstats.sys = memstats.heap_sys + memstats.stacks_sys + memstats.mspan_sys +
		memstats.mcache_sys + memstats.buckhash_sys + memstats.gc_sys + memstats.other_sys

	// We also count stacks_inuse as sys memory.
	memstats.sys += memstats.stacks_inuse

	// Calculate memory allocator stats.
	// During program execution we only count number of frees and amount of freed memory.
	// Current number of alive objects in the heap and amount of alive heap memory
	// are calculated by scanning all spans.
	// Total number of mallocs is calculated as number of frees plus number of alive objects.
	// Similarly, total amount of allocated memory is calculated as amount of freed memory
	// plus amount of alive heap memory.
	memstats.alloc = 0
	memstats.total_alloc = 0
	memstats.nmalloc = 0
	memstats.nfree = 0
	for i := 0; i < len(memstats.by_size); i++ {
		memstats.by_size[i].nmalloc = 0
		memstats.by_size[i].nfree = 0
	}

	// Aggregate local stats.
	cachestats()

	// Collect allocation stats. This is safe and consistent
	// because the world is stopped.
	var smallFree, totalAlloc, totalFree uint64
	// Collect per-spanclass stats.
	for spc := range mheap_.central {
		// The mcaches are now empty, so mcentral stats are
		// up-to-date.
		c := &mheap_.central[spc].mcentral
		memstats.nmalloc += c.nmalloc
		i := spanClass(spc).sizeclass()
		memstats.by_size[i].nmalloc += c.nmalloc
		totalAlloc += c.nmalloc * uint64(class_to_size[i])
	}
	// Collect per-sizeclass stats.
	for i := 0; i < _NumSizeClasses; i++ {
		if i == 0 {
			memstats.nmalloc += mheap_.nlargealloc
			totalAlloc += mheap_.largealloc
			totalFree += mheap_.largefree
			memstats.nfree += mheap_.nlargefree
			continue
		}

		// The mcache stats have been flushed to mheap_.
		memstats.nfree += mheap_.nsmallfree[i]
		memstats.by_size[i].nfree = mheap_.nsmallfree[i]
		smallFree += mheap_.nsmallfree[i] * uint64(class_to_size[i])
	}
	totalFree += smallFree

	memstats.nfree += memstats.tinyallocs
	memstats.nmalloc += memstats.tinyallocs

	// Calculate derived stats.
	memstats.total_alloc = totalAlloc
	memstats.alloc = totalAlloc - totalFree
	memstats.heap_alloc = memstats.alloc
	memstats.heap_objects = memstats.nmalloc - memstats.nfree
}

// cachestats flushes all mcache stats.
//
// The world must be stopped.
//
//go:nowritebarrier
func cachestats() {
	for _, p := range allp {
		c := p.mcache
		if c == nil {
			continue
		}
		purgecachedstats(c)
	}
}

// flushmcache flushes the mcache of allp[i].
//
// The world must be stopped.
//
//go:nowritebarrier
func flushmcache(i int) {
	p := allp[i]
	c := p.mcache
	if c == nil {
		return
	}
	c.releaseAll()
	stackcache_clear(c)
}

// flushallmcaches flushes the mcaches of all Ps.
//
// The world must be stopped.
//
//go:nowritebarrier
func flushallmcaches() {
	for i := 0; i < int(gomaxprocs); i++ {
		flushmcache(i)
	}
}

//go:nosplit
func purgecachedstats(c *mcache) {
	// Protected by either heap or GC lock.
	h := &mheap_
	memstats.heap_scan += uint64(c.local_scan)
	c.local_scan = 0
	memstats.tinyallocs += uint64(c.local_tinyallocs)
	c.local_tinyallocs = 0
	h.largefree += uint64(c.local_largefree)
	c.local_largefree = 0
	h.nlargefree += uint64(c.local_nlargefree)
	c.local_nlargefree = 0
	for i := 0; i < len(c.local_nsmallfree); i++ {
		h.nsmallfree[i] += uint64(c.local_nsmallfree[i])
		c.local_nsmallfree[i] = 0
	}
}

// Atomically increases a given *system* memory stat. We are counting on this
// stat never overflowing a uintptr, so this function must only be used for
// system memory stats.
//
// The current implementation for little endian architectures is based on
// xadduintptr(), which is less than ideal: xadd64() should really be used.
// Using xadduintptr() is a stop-gap solution until arm supports xadd64() that
// doesn't use locks.  (Locks are a problem as they require a valid G, which
// restricts their useability.)
//
// A side-effect of using xadduintptr() is that we need to check for
// overflow errors.
//go:nosplit
func mSysStatInc(sysStat *uint64, n uintptr) {
	if sysStat == nil {
		return
	}
	if sys.BigEndian {
		atomic.Xadd64(sysStat, int64(n))
		return
	}
	if val := atomic.Xadduintptr((*uintptr)(unsafe.Pointer(sysStat)), n); val < n {
		print("runtime: stat overflow: val ", val, ", n ", n, "\n")
		exit(2)
	}
}

// Atomically decreases a given *system* memory stat. Same comments as
// mSysStatInc apply.
//go:nosplit
func mSysStatDec(sysStat *uint64, n uintptr) {
	if sysStat == nil {
		return
	}
	if sys.BigEndian {
		atomic.Xadd64(sysStat, -int64(n))
		return
	}
	if val := atomic.Xadduintptr((*uintptr)(unsafe.Pointer(sysStat)), uintptr(-int64(n))); val+n < n {
		print("runtime: stat underflow: val ", val, ", n ", n, "\n")
		exit(2)
	}
}
