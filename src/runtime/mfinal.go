// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Garbage collector: finalizers and block profiling.

package runtime

import (
	"runtime/internal/atomic"
	"runtime/internal/sys"
	"unsafe"
)

// finblock is an array of finalizers to be executed. finblocks are
// arranged in a linked list for the finalizer queue.
//
// finblock is allocated from non-GC'd memory, so any heap pointers
// must be specially handled. GC currently assumes that the finalizer
// queue does not grow during marking (but it can shrink).
//
//go:notinheap
type finblock struct {
	alllink *finblock
	next    *finblock
	cnt     uint32
	_       int32
	fin     [(_FinBlockSize - 2*sys.PtrSize - 2*4) / unsafe.Sizeof(finalizer{})]finalizer
}

var finlock mutex  // protects the following variables
var fing *g        // goroutine that runs finalizers
var finq *finblock // list of finalizers that are to be executed
var finc *finblock // cache of free blocks
var finptrmask [_FinBlockSize / sys.PtrSize / 8]byte
var fingwait bool
var fingwake bool
var allfin *finblock // list of all blocks

// NOTE: Layout known to queuefinalizer.
type finalizer struct {
	fn   *funcval       // function to call (may be a heap pointer)
	arg  unsafe.Pointer // ptr to object (may be a heap pointer)
	nret uintptr        // bytes of return values from fn
	fint *_type         // type of first argument of fn
	ot   *ptrtype       // type of ptr to object (may be a heap pointer)
}

var finalizer1 = [...]byte{
	// Each Finalizer is 5 words, ptr ptr INT ptr ptr (INT = uintptr here)
	// Each byte describes 8 words.
	// Need 8 Finalizers described by 5 bytes before pattern repeats:
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	// aka
	//
	//	ptr ptr INT ptr ptr ptr ptr INT
	//	ptr ptr ptr ptr INT ptr ptr ptr
	//	ptr INT ptr ptr ptr ptr INT ptr
	//	ptr ptr ptr INT ptr ptr ptr ptr
	//	INT ptr ptr ptr ptr INT ptr ptr
	//
	// Assumptions about Finalizer layout checked below.
	1<<0 | 1<<1 | 0<<2 | 1<<3 | 1<<4 | 1<<5 | 1<<6 | 0<<7,
	1<<0 | 1<<1 | 1<<2 | 1<<3 | 0<<4 | 1<<5 | 1<<6 | 1<<7,
	1<<0 | 0<<1 | 1<<2 | 1<<3 | 1<<4 | 1<<5 | 0<<6 | 1<<7,
	1<<0 | 1<<1 | 1<<2 | 0<<3 | 1<<4 | 1<<5 | 1<<6 | 1<<7,
	0<<0 | 1<<1 | 1<<2 | 1<<3 | 1<<4 | 0<<5 | 1<<6 | 1<<7,
}

func queuefinalizer(p unsafe.Pointer, fn *funcval, nret uintptr, fint *_type, ot *ptrtype) {
	if gcphase != _GCoff {
		// Currently we assume that the finalizer queue won't
		// grow during marking so we don't have to rescan it
		// during mark termination. If we ever need to lift
		// this assumption, we can do it by adding the
		// necessary barriers to queuefinalizer (which it may
		// have automatically).
		throw("queuefinalizer during GC")
	}

	lock(&finlock)
	if finq == nil || finq.cnt == uint32(len(finq.fin)) {
		if finc == nil {
			finc = (*finblock)(persistentalloc(_FinBlockSize, 0, &memstats.gc_sys))
			finc.alllink = allfin
			allfin = finc
			if finptrmask[0] == 0 {
				// Build pointer mask for Finalizer array in block.
				// Check assumptions made in finalizer1 array above.
				if (unsafe.Sizeof(finalizer{}) != 5*sys.PtrSize ||
					unsafe.Offsetof(finalizer{}.fn) != 0 ||
					unsafe.Offsetof(finalizer{}.arg) != sys.PtrSize ||
					unsafe.Offsetof(finalizer{}.nret) != 2*sys.PtrSize ||
					unsafe.Offsetof(finalizer{}.fint) != 3*sys.PtrSize ||
					unsafe.Offsetof(finalizer{}.ot) != 4*sys.PtrSize) {
					throw("finalizer out of sync")
				}
				for i := range finptrmask {
					finptrmask[i] = finalizer1[i%len(finalizer1)]
				}
			}
		}
		block := finc
		finc = block.next
		block.next = finq
		finq = block
	}
	f := &finq.fin[finq.cnt]
	atomic.Xadd(&finq.cnt, +1) // Sync with markroots
	f.fn = fn
	f.nret = nret
	f.fint = fint
	f.ot = ot
	f.arg = p
	fingwake = true
	unlock(&finlock)
}

//go:nowritebarrier
func iterate_finq(callback func(*funcval, unsafe.Pointer, uintptr, *_type, *ptrtype)) {
	for fb := allfin; fb != nil; fb = fb.alllink {
		for i := uint32(0); i < fb.cnt; i++ {
			f := &fb.fin[i]
			callback(f.fn, f.arg, f.nret, f.fint, f.ot)
		}
	}
}

func wakefing() *g {
	var res *g
	lock(&finlock)
	if fingwait && fingwake {
		fingwait = false
		fingwake = false
		res = fing
	}
	unlock(&finlock)
	return res
}

var (
	fingCreate  uint32
	fingRunning bool
)

func createfing() {
	// start the finalizer goroutine exactly once
	if fingCreate == 0 && atomic.Cas(&fingCreate, 0, 1) {
		go runfinq()
	}
}

// This is the goroutine that runs all of the finalizers
func runfinq() {
	var (
		frame    unsafe.Pointer
		framecap uintptr
	)

	for {
		lock(&finlock)
		fb := finq
		finq = nil
		if fb == nil {
			gp := getg()
			fing = gp
			fingwait = true
			goparkunlock(&finlock, waitReasonFinalizerWait, traceEvGoBlock, 1)
			continue
		}
		unlock(&finlock)
		if raceenabled {
			racefingo()
		}
		for fb != nil {
			for i := fb.cnt; i > 0; i-- {
				f := &fb.fin[i-1]

				framesz := unsafe.Sizeof((interface{})(nil)) + f.nret
				if framecap < framesz {
					// The frame does not contain pointers interesting for GC,
					// all not yet finalized objects are stored in finq.
					// If we do not mark it as FlagNoScan,
					// the last finalized object is not collected.
					frame = mallocgc(framesz, nil, true)
					framecap = framesz
				}

				if f.fint == nil {
					throw("missing type in runfinq")
				}
				// frame is effectively uninitialized
				// memory. That means we have to clear
				// it before writing to it to avoid
				// confusing the write barrier.
				*(*[2]uintptr)(frame) = [2]uintptr{}
				switch f.fint.kind & kindMask {
				case kindPtr:
					// direct use of pointer
					*(*unsafe.Pointer)(frame) = f.arg
				case kindInterface:
					ityp := (*interfacetype)(unsafe.Pointer(f.fint))
					// set up with empty interface
					(*eface)(frame)._type = &f.ot.typ
					(*eface)(frame).data = f.arg
					if len(ityp.mhdr) != 0 {
						// convert to interface with methods
						// this conversion is guaranteed to succeed - we checked in SetFinalizer
						*(*iface)(frame) = assertE2I(ityp, *(*eface)(frame))
					}
				default:
					throw("bad kind in runfinq")
				}
				fingRunning = true
				reflectcall(nil, unsafe.Pointer(f.fn), frame, uint32(framesz), uint32(framesz))
				fingRunning = false

				// Drop finalizer queue heap references
				// before hiding them from markroot.
				// This also ensures these will be
				// clear if we reuse the finalizer.
				f.fn = nil
				f.arg = nil
				f.ot = nil
				atomic.Store(&fb.cnt, i-1)
			}
			next := fb.next
			lock(&finlock)
			fb.next = finc
			finc = fb
			unlock(&finlock)
			fb = next
		}
	}
}

// SetFinalizer 将与 obj 关联的 finalizer 设置为提供的 finalizer 函数。
// 当垃圾收集器发现一个带有相关 finalizer 的不可达块时，它会清除关联，
// 并在单独的 goroutine 中运行终结器(obj)。这使得 obj 再次可达，但现在没有关联的 finalizer。
// 假设没有再次调用 SetFinalizer，那么下次垃圾收集器看到obj不可达时，就会释放obj。
//
// SetFinalizer(obj, nil) 清除 obj 关联的所有 finalizer  。
//
// obj 参数必须是指向通过调用 new、获取复合文字的地址或获取局部变量的地址
// 来分配的对象的指针。
// finalizer 必须是一个接受单个参数的函数，可以将 obj 的类型赋给该参数，
// 并且可以具有任意忽略的返回值。如果这两个条件都不成立，SetFinalizer 可能会中止程序。
//
// Finalizers 按依赖顺序运行:如果 A 指向 B，两者都有 finalizers，
// 否则无法到达，只有 A 的终结器运行；一旦 A 被释放，B 的终结器就可以运行了。
// 如果循环结构包含带有 finalizer 的块，则循环不能保证被垃圾收集，
// finalizer 也不能保证运行，因为没有考虑依赖关系的排序。
//
// finalizer 被安排在程序无法再到达 obj 指向的对象之后的任意时间运行。
// 无法保证 finalizer 会在程序退出之前运行，
// 因此通常它们仅用于在长时间运行程序期间释放与对象相关联的非内存资源。
// 例如，当程序放弃 os.File 时，os.File 可以使用 finalizer 来关闭相关的
// 操作系统文件描述符。但依赖 finalizer 来刷新内存中的 
// I/O buffer (如bufio.Writer) 是错误的。因为在程序退出时不会刷新缓冲区。
//
// 如果 *obj 的大小为零字节，则不能保证 finalizer 会运行。
//
// 不能保证 finalizer 会为包级变量的初始值设定项中分配的对象运行。
// 这些对象可以是链接器分配的，而不是堆分配的。
//
// 一旦对象变得不可访问，finalizer 就可以运行。
// 为了正确使用 finalizer，程序必须确保对象在不再需要之前是可达的。
// 存储在全局变量中的对象，或者可以通过跟踪全局变量中的指针找到的对象，都是可达的。
// 对于其他对象，将对象传递给 KeepAlive 函数的调用，以标记函数中对象必须可到达的最后一点。
//
// 例如，如果 p 指向一个 struct，如 os.File，
// 它包含一个文件描述符 d，p 有一个 finalizer 来关闭那个文件描述符，
// 如果函数中最后一次使用 p 是调用 syscall.Write(p.d, buf, size)，
// 那么程序一进入 syscall.Write 就可能无法到达 p。
// finalizer 可能会在此时运行，关闭 p.d，导致 syscall.Write 失败，
// 因为它正在向一个关闭了的文件描述符写入
// (或者，更糟糕的是，由不同的 goroutine 打开的完全不同的文件描述符)。
// 若要避免此问题，请在调用 syscall.Write 后调用 runtime.KeepAlive(p)。
//
// 单个 goroutine 按顺序运行一个程序的所有 finalizer。
// 如果 finalizer 必须运行很长时间，它应该通过启动一个新的 goroutine 来完成。
func SetFinalizer(obj interface{}, finalizer interface{}) {
	if debug.sbrk != 0 {
		// debug.sbrk never frees memory, so no finalizers run
		// (and we don't have the data structures to record them).
		return
	}
	e := efaceOf(&obj)
	etyp := e._type
	if etyp == nil {
		throw("runtime.SetFinalizer: first argument is nil")
	}
	if etyp.kind&kindMask != kindPtr {
		throw("runtime.SetFinalizer: first argument is " + etyp.string() + ", not pointer")
	}
	ot := (*ptrtype)(unsafe.Pointer(etyp))
	if ot.elem == nil {
		throw("nil elem type!")
	}

	// find the containing object
	base, _, _ := findObject(uintptr(e.data), 0, 0)

	if base == 0 {
		// 0-length objects are okay.
		if e.data == unsafe.Pointer(&zerobase) {
			return
		}

		// Global initializers might be linker-allocated.
		//	var Foo = &Object{}
		//	func main() {
		//		runtime.SetFinalizer(Foo, nil)
		//	}
		// The relevant segments are: noptrdata, data, bss, noptrbss.
		// We cannot assume they are in any order or even contiguous,
		// due to external linking.
		for datap := &firstmoduledata; datap != nil; datap = datap.next {
			if datap.noptrdata <= uintptr(e.data) && uintptr(e.data) < datap.enoptrdata ||
				datap.data <= uintptr(e.data) && uintptr(e.data) < datap.edata ||
				datap.bss <= uintptr(e.data) && uintptr(e.data) < datap.ebss ||
				datap.noptrbss <= uintptr(e.data) && uintptr(e.data) < datap.enoptrbss {
				return
			}
		}
		throw("runtime.SetFinalizer: pointer not in allocated block")
	}

	if uintptr(e.data) != base {
		// As an implementation detail we allow to set finalizers for an inner byte
		// of an object if it could come from tiny alloc (see mallocgc for details).
		if ot.elem == nil || ot.elem.ptrdata != 0 || ot.elem.size >= maxTinySize {
			throw("runtime.SetFinalizer: pointer not at beginning of allocated block")
		}
	}

	f := efaceOf(&finalizer)
	ftyp := f._type
	if ftyp == nil {
		// switch to system stack and remove finalizer
		systemstack(func() {
			removefinalizer(e.data)
		})
		return
	}

	if ftyp.kind&kindMask != kindFunc {
		throw("runtime.SetFinalizer: second argument is " + ftyp.string() + ", not a function")
	}
	ft := (*functype)(unsafe.Pointer(ftyp))
	if ft.dotdotdot() {
		throw("runtime.SetFinalizer: cannot pass " + etyp.string() + " to finalizer " + ftyp.string() + " because dotdotdot")
	}
	if ft.inCount != 1 {
		throw("runtime.SetFinalizer: cannot pass " + etyp.string() + " to finalizer " + ftyp.string())
	}
	fint := ft.in()[0]
	switch {
	case fint == etyp:
		// ok - same type
		goto okarg
	case fint.kind&kindMask == kindPtr:
		if (fint.uncommon() == nil || etyp.uncommon() == nil) && (*ptrtype)(unsafe.Pointer(fint)).elem == ot.elem {
			// ok - not same type, but both pointers,
			// one or the other is unnamed, and same element type, so assignable.
			goto okarg
		}
	case fint.kind&kindMask == kindInterface:
		ityp := (*interfacetype)(unsafe.Pointer(fint))
		if len(ityp.mhdr) == 0 {
			// ok - satisfies empty interface
			goto okarg
		}
		if _, ok := assertE2I2(ityp, *efaceOf(&obj)); ok {
			goto okarg
		}
	}
	throw("runtime.SetFinalizer: cannot pass " + etyp.string() + " to finalizer " + ftyp.string())
okarg:
	// compute size needed for return parameters
	nret := uintptr(0)
	for _, t := range ft.out() {
		nret = alignUp(nret, uintptr(t.align)) + uintptr(t.size)
	}
	nret = alignUp(nret, sys.PtrSize)

	// make sure we have a finalizer goroutine
	createfing()

	systemstack(func() {
		if !addfinalizer(e.data, (*funcval)(f.data), nret, fint, ot) {
			throw("runtime.SetFinalizer: finalizer already set")
		}
	})
}

// Mark KeepAlive as noinline so that it is easily detectable as an intrinsic.
//go:noinline

// KeepAlive 将其参数标记为当前可达。
// 这确保了在程序中调用 KeepAlive 的点之前，对象不会被释放，其 finalizer 也不会运行。
//
// 一个非常简单的例子显示了哪里需要保持活动:
// 	type File struct { d int }
// 	d, err := syscall.Open("/file/path", syscall.O_RDONLY, 0)
// 	// ... do something if err != nil ...
// 	p := &File{d}
// 	runtime.SetFinalizer(p, func(p *File) { syscall.Close(p.d) })
// 	var buf [10]byte
// 	n, err := syscall.Read(p.d, buf[:])
// 	// 确保在 Read 返回之前，p 尚未 finalized。
// 	runtime.KeepAlive(p)
// 	// 在这个点之后，p 不再使用。
// 
// 没有 KeepAlive 调用，finalizer 可以在 syscall.Read 开始时运行。
// 在 syscall.Read 进行真正系统调用之前关闭文件描述符。
func KeepAlive(x interface{}) {
	// Introduce a use of x that the compiler can't eliminate.
	// This makes sure x is alive on entry. We need x to be alive
	// on entry for "defer runtime.KeepAlive(x)"; see issue 21402.
	if cgoAlwaysFalse {
		println(x)
	}
}
