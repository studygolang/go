// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"runtime/internal/atomic"
	"runtime/internal/sys"
	"unsafe"
)

// The code in this file implements stack trace walking for all architectures.
// The most important fact about a given architecture is whether it uses a link register.
// On systems with link registers, the prologue for a non-leaf function stores the
// incoming value of LR at the bottom of the newly allocated stack frame.
// On systems without link registers, the architecture pushes a return PC during
// the call instruction, so the return PC ends up above the stack frame.
// In this file, the return PC is always called LR, no matter how it was found.
//
// To date, the opposite of a link register architecture is an x86 architecture.
// This code may need to change if some other kind of non-link-register
// architecture comes along.
//
// The other important fact is the size of a pointer: on 32-bit systems the LR
// takes up only 4 bytes on the stack, while on 64-bit systems it takes up 8 bytes.
// Typically this is ptrSize.
//
// As an exception, amd64p32 had ptrSize == 4 but the CALL instruction still
// stored an 8-byte return PC onto the stack. To accommodate this, we used regSize
// as the size of the architecture-pushed return PC.
//
// usesLR is defined below in terms of minFrameSize, which is defined in
// arch_$GOARCH.go. ptrSize and regSize are defined in stubs.go.

const usesLR = sys.MinFrameSize > 0

var skipPC uintptr

func tracebackinit() {
	// Go variable initialization happens late during runtime startup.
	// Instead of initializing the variables above in the declarations,
	// schedinit calls this function so that the variables are
	// initialized and available earlier in the startup sequence.
	skipPC = funcPC(skipPleaseUseCallersFrames)
}

// Traceback over the deferred function calls.
// Report them like calls that have been invoked but not started executing yet.
func tracebackdefers(gp *g, callback func(*stkframe, unsafe.Pointer) bool, v unsafe.Pointer) {
	var frame stkframe
	for d := gp._defer; d != nil; d = d.link {
		fn := d.fn
		if fn == nil {
			// Defer of nil function. Args don't matter.
			frame.pc = 0
			frame.fn = funcInfo{}
			frame.argp = 0
			frame.arglen = 0
			frame.argmap = nil
		} else {
			frame.pc = fn.fn
			f := findfunc(frame.pc)
			if !f.valid() {
				print("runtime: unknown pc in defer ", hex(frame.pc), "\n")
				throw("unknown pc")
			}
			frame.fn = f
			frame.argp = uintptr(deferArgs(d))
			var ok bool
			frame.arglen, frame.argmap, ok = getArgInfoFast(f, true)
			if !ok {
				frame.arglen, frame.argmap = getArgInfo(&frame, f, true, fn)
			}
		}
		frame.continpc = frame.pc
		if !callback((*stkframe)(noescape(unsafe.Pointer(&frame))), v) {
			return
		}
	}
}

const sizeofSkipFunction = 256

// This function is defined in asm.s to be sizeofSkipFunction bytes long.
func skipPleaseUseCallersFrames()

// Generic traceback. Handles runtime stack prints (pcbuf == nil),
// the runtime.Callers function (pcbuf != nil), as well as the garbage
// collector (callback != nil).  A little clunky to merge these, but avoids
// duplicating the code and all its subtlety.
//
// The skip argument is only valid with pcbuf != nil and counts the number
// of logical frames to skip rather than physical frames (with inlining, a
// PC in pcbuf can represent multiple calls). If a PC is partially skipped
// and max > 1, pcbuf[1] will be runtime.skipPleaseUseCallersFrames+N where
// N indicates the number of logical frames to skip in pcbuf[0].
func gentraceback(pc0, sp0, lr0 uintptr, gp *g, skip int, pcbuf *uintptr, max int, callback func(*stkframe, unsafe.Pointer) bool, v unsafe.Pointer, flags uint) int {
	if skip > 0 && callback != nil {
		throw("gentraceback callback cannot be used with non-zero skip")
	}

	// Don't call this "g"; it's too easy get "g" and "gp" confused.
	if ourg := getg(); ourg == gp && ourg == ourg.m.curg {
		// The starting sp has been passed in as a uintptr, and the caller may
		// have other uintptr-typed stack references as well.
		// If during one of the calls that got us here or during one of the
		// callbacks below the stack must be grown, all these uintptr references
		// to the stack will not be updated, and gentraceback will continue
		// to inspect the old stack memory, which may no longer be valid.
		// Even if all the variables were updated correctly, it is not clear that
		// we want to expose a traceback that begins on one stack and ends
		// on another stack. That could confuse callers quite a bit.
		// Instead, we require that gentraceback and any other function that
		// accepts an sp for the current goroutine (typically obtained by
		// calling getcallersp) must not run on that goroutine's stack but
		// instead on the g0 stack.
		throw("gentraceback cannot trace user goroutine on its own stack")
	}
	level, _, _ := gotraceback()

	var ctxt *funcval // Context pointer for unstarted goroutines. See issue #25897.

	if pc0 == ^uintptr(0) && sp0 == ^uintptr(0) { // Signal to fetch saved values from gp.
		if gp.syscallsp != 0 {
			pc0 = gp.syscallpc
			sp0 = gp.syscallsp
			if usesLR {
				lr0 = 0
			}
		} else {
			pc0 = gp.sched.pc
			sp0 = gp.sched.sp
			if usesLR {
				lr0 = gp.sched.lr
			}
			ctxt = (*funcval)(gp.sched.ctxt)
		}
	}

	nprint := 0
	var frame stkframe
	frame.pc = pc0
	frame.sp = sp0
	if usesLR {
		frame.lr = lr0
	}
	waspanic := false
	cgoCtxt := gp.cgoCtxt
	printing := pcbuf == nil && callback == nil

	// If the PC is zero, it's likely a nil function call.
	// Start in the caller's frame.
	if frame.pc == 0 {
		if usesLR {
			frame.pc = *(*uintptr)(unsafe.Pointer(frame.sp))
			frame.lr = 0
		} else {
			frame.pc = uintptr(*(*sys.Uintreg)(unsafe.Pointer(frame.sp)))
			frame.sp += sys.RegSize
		}
	}

	f := findfunc(frame.pc)
	if !f.valid() {
		if callback != nil || printing {
			print("runtime: unknown pc ", hex(frame.pc), "\n")
			tracebackHexdump(gp.stack, &frame, 0)
		}
		if callback != nil {
			throw("unknown pc")
		}
		return 0
	}
	frame.fn = f

	var cache pcvalueCache

	lastFuncID := funcID_normal
	n := 0
	for n < max {
		// Typically:
		//	pc is the PC of the running function.
		//	sp is the stack pointer at that program counter.
		//	fp is the frame pointer (caller's stack pointer) at that program counter, or nil if unknown.
		//	stk is the stack containing sp.
		//	The caller's program counter is lr, unless lr is zero, in which case it is *(uintptr*)sp.
		f = frame.fn
		if f.pcsp == 0 {
			// No frame information, must be external function, like race support.
			// See golang.org/issue/13568.
			break
		}

		// Found an actual function.
		// Derive frame pointer and link register.
		if frame.fp == 0 {
			// Jump over system stack transitions. If we're on g0 and there's a user
			// goroutine, try to jump. Otherwise this is a regular call.
			if flags&_TraceJumpStack != 0 && gp == gp.m.g0 && gp.m.curg != nil {
				switch f.funcID {
				case funcID_morestack:
					// morestack does not return normally -- newstack()
					// gogo's to curg.sched. Match that.
					// This keeps morestack() from showing up in the backtrace,
					// but that makes some sense since it'll never be returned
					// to.
					frame.pc = gp.m.curg.sched.pc
					frame.fn = findfunc(frame.pc)
					f = frame.fn
					frame.sp = gp.m.curg.sched.sp
					cgoCtxt = gp.m.curg.cgoCtxt
				case funcID_systemstack:
					// systemstack returns normally, so just follow the
					// stack transition.
					frame.sp = gp.m.curg.sched.sp
					cgoCtxt = gp.m.curg.cgoCtxt
				}
			}
			frame.fp = frame.sp + uintptr(funcspdelta(f, frame.pc, &cache))
			if !usesLR {
				// On x86, call instruction pushes return PC before entering new function.
				frame.fp += sys.RegSize
			}
		}
		var flr funcInfo
		if topofstack(f, gp.m != nil && gp == gp.m.g0) {
			frame.lr = 0
			flr = funcInfo{}
		} else if usesLR && f.funcID == funcID_jmpdefer {
			// jmpdefer modifies SP/LR/PC non-atomically.
			// If a profiling interrupt arrives during jmpdefer,
			// the stack unwind may see a mismatched register set
			// and get confused. Stop if we see PC within jmpdefer
			// to avoid that confusion.
			// See golang.org/issue/8153.
			if callback != nil {
				throw("traceback_arm: found jmpdefer when tracing with callback")
			}
			frame.lr = 0
		} else {
			var lrPtr uintptr
			if usesLR {
				if n == 0 && frame.sp < frame.fp || frame.lr == 0 {
					lrPtr = frame.sp
					frame.lr = *(*uintptr)(unsafe.Pointer(lrPtr))
				}
			} else {
				if frame.lr == 0 {
					lrPtr = frame.fp - sys.RegSize
					frame.lr = uintptr(*(*sys.Uintreg)(unsafe.Pointer(lrPtr)))
				}
			}
			flr = findfunc(frame.lr)
			if !flr.valid() {
				// This happens if you get a profiling interrupt at just the wrong time.
				// In that context it is okay to stop early.
				// But if callback is set, we're doing a garbage collection and must
				// get everything, so crash loudly.
				doPrint := printing
				if doPrint && gp.m.incgo && f.funcID == funcID_sigpanic {
					// We can inject sigpanic
					// calls directly into C code,
					// in which case we'll see a C
					// return PC. Don't complain.
					doPrint = false
				}
				if callback != nil || doPrint {
					print("runtime: unexpected return pc for ", funcname(f), " called from ", hex(frame.lr), "\n")
					tracebackHexdump(gp.stack, &frame, lrPtr)
				}
				if callback != nil {
					throw("unknown caller pc")
				}
			}
		}

		frame.varp = frame.fp
		if !usesLR {
			// On x86, call instruction pushes return PC before entering new function.
			frame.varp -= sys.RegSize
		}

		// If framepointer_enabled and there's a frame, then
		// there's a saved bp here.
		if frame.varp > frame.sp && (framepointer_enabled && GOARCH == "amd64" || GOARCH == "arm64") {
			frame.varp -= sys.RegSize
		}

		// Derive size of arguments.
		// Most functions have a fixed-size argument block,
		// so we can use metadata about the function f.
		// Not all, though: there are some variadic functions
		// in package runtime and reflect, and for those we use call-specific
		// metadata recorded by f's caller.
		if callback != nil || printing {
			frame.argp = frame.fp + sys.MinFrameSize
			var ok bool
			frame.arglen, frame.argmap, ok = getArgInfoFast(f, callback != nil)
			if !ok {
				frame.arglen, frame.argmap = getArgInfo(&frame, f, callback != nil, ctxt)
			}
		}
		ctxt = nil // ctxt is only needed to get arg maps for the topmost frame

		// Determine frame's 'continuation PC', where it can continue.
		// Normally this is the return address on the stack, but if sigpanic
		// is immediately below this function on the stack, then the frame
		// stopped executing due to a trap, and frame.pc is probably not
		// a safe point for looking up liveness information. In this panicking case,
		// the function either doesn't return at all (if it has no defers or if the
		// defers do not recover) or it returns from one of the calls to
		// deferproc a second time (if the corresponding deferred func recovers).
		// In the latter case, use a deferreturn call site as the continuation pc.
		frame.continpc = frame.pc
		if waspanic {
			if frame.fn.deferreturn != 0 {
				frame.continpc = frame.fn.entry + uintptr(frame.fn.deferreturn) + 1
				// Note: this may perhaps keep return variables alive longer than
				// strictly necessary, as we are using "function has a defer statement"
				// as a proxy for "function actually deferred something". It seems
				// to be a minor drawback. (We used to actually look through the
				// gp._defer for a defer corresponding to this function, but that
				// is hard to do with defer records on the stack during a stack copy.)
				// Note: the +1 is to offset the -1 that
				// stack.go:getStackMap does to back up a return
				// address make sure the pc is in the CALL instruction.
			} else {
				frame.continpc = 0
			}
		}

		if callback != nil {
			if !callback((*stkframe)(noescape(unsafe.Pointer(&frame))), v) {
				return n
			}
		}

		if pcbuf != nil {
			pc := frame.pc
			// backup to CALL instruction to read inlining info (same logic as below)
			tracepc := pc
			// Normally, pc is a return address. In that case, we want to look up
			// file/line information using pc-1, because that is the pc of the
			// call instruction (more precisely, the last byte of the call instruction).
			// Callers expect the pc buffer to contain return addresses and do the
			// same -1 themselves, so we keep pc unchanged.
			// When the pc is from a signal (e.g. profiler or segv) then we want
			// to look up file/line information using pc, and we store pc+1 in the
			// pc buffer so callers can unconditionally subtract 1 before looking up.
			// See issue 34123.
			// The pc can be at function entry when the frame is initialized without
			// actually running code, like runtime.mstart.
			if (n == 0 && flags&_TraceTrap != 0) || waspanic || pc == f.entry {
				pc++
			} else {
				tracepc--
			}

			// If there is inlining info, record the inner frames.
			if inldata := funcdata(f, _FUNCDATA_InlTree); inldata != nil {
				inltree := (*[1 << 20]inlinedCall)(inldata)
				for {
					ix := pcdatavalue(f, _PCDATA_InlTreeIndex, tracepc, &cache)
					if ix < 0 {
						break
					}
					if inltree[ix].funcID == funcID_wrapper && elideWrapperCalling(lastFuncID) {
						// ignore wrappers
					} else if skip > 0 {
						skip--
					} else if n < max {
						(*[1 << 20]uintptr)(unsafe.Pointer(pcbuf))[n] = pc
						n++
					}
					lastFuncID = inltree[ix].funcID
					// Back up to an instruction in the "caller".
					tracepc = frame.fn.entry + uintptr(inltree[ix].parentPc)
					pc = tracepc + 1
				}
			}
			// Record the main frame.
			if f.funcID == funcID_wrapper && elideWrapperCalling(lastFuncID) {
				// Ignore wrapper functions (except when they trigger panics).
			} else if skip > 0 {
				skip--
			} else if n < max {
				(*[1 << 20]uintptr)(unsafe.Pointer(pcbuf))[n] = pc
				n++
			}
			lastFuncID = f.funcID
			n-- // offset n++ below
		}

		if printing {
			// assume skip=0 for printing.
			//
			// Never elide wrappers if we haven't printed
			// any frames. And don't elide wrappers that
			// called panic rather than the wrapped
			// function. Otherwise, leave them out.

			// backup to CALL instruction to read inlining info (same logic as below)
			tracepc := frame.pc
			if (n > 0 || flags&_TraceTrap == 0) && frame.pc > f.entry && !waspanic {
				tracepc--
			}
			// If there is inlining info, print the inner frames.
			if inldata := funcdata(f, _FUNCDATA_InlTree); inldata != nil {
				inltree := (*[1 << 20]inlinedCall)(inldata)
				for {
					ix := pcdatavalue(f, _PCDATA_InlTreeIndex, tracepc, nil)
					if ix < 0 {
						break
					}
					if (flags&_TraceRuntimeFrames) != 0 || showframe(f, gp, nprint == 0, inltree[ix].funcID, lastFuncID) {
						name := funcnameFromNameoff(f, inltree[ix].func_)
						file, line := funcline(f, tracepc)
						print(name, "(...)\n")
						print("\t", file, ":", line, "\n")
						nprint++
					}
					lastFuncID = inltree[ix].funcID
					// Back up to an instruction in the "caller".
					tracepc = frame.fn.entry + uintptr(inltree[ix].parentPc)
				}
			}
			if (flags&_TraceRuntimeFrames) != 0 || showframe(f, gp, nprint == 0, f.funcID, lastFuncID) {
				// Print during crash.
				//	main(0x1, 0x2, 0x3)
				//		/home/rsc/go/src/runtime/x.go:23 +0xf
				//
				name := funcname(f)
				file, line := funcline(f, tracepc)
				if name == "runtime.gopanic" {
					name = "panic"
				}
				print(name, "(")
				argp := (*[100]uintptr)(unsafe.Pointer(frame.argp))
				for i := uintptr(0); i < frame.arglen/sys.PtrSize; i++ {
					if i >= 10 {
						print(", ...")
						break
					}
					if i != 0 {
						print(", ")
					}
					print(hex(argp[i]))
				}
				print(")\n")
				print("\t", file, ":", line)
				if frame.pc > f.entry {
					print(" +", hex(frame.pc-f.entry))
				}
				if gp.m != nil && gp.m.throwing > 0 && gp == gp.m.curg || level >= 2 {
					print(" fp=", hex(frame.fp), " sp=", hex(frame.sp), " pc=", hex(frame.pc))
				}
				print("\n")
				nprint++
			}
			lastFuncID = f.funcID
		}
		n++

		if f.funcID == funcID_cgocallback_gofunc && len(cgoCtxt) > 0 {
			ctxt := cgoCtxt[len(cgoCtxt)-1]
			cgoCtxt = cgoCtxt[:len(cgoCtxt)-1]

			// skip only applies to Go frames.
			// callback != nil only used when we only care
			// about Go frames.
			if skip == 0 && callback == nil {
				n = tracebackCgoContext(pcbuf, printing, ctxt, n, max)
			}
		}

		waspanic = f.funcID == funcID_sigpanic
		injectedCall := waspanic || f.funcID == funcID_asyncPreempt

		// Do not unwind past the bottom of the stack.
		if !flr.valid() {
			break
		}

		// Unwind to next frame.
		frame.fn = flr
		frame.pc = frame.lr
		frame.lr = 0
		frame.sp = frame.fp
		frame.fp = 0
		frame.argmap = nil

		// On link register architectures, sighandler saves the LR on stack
		// before faking a call.
		if usesLR && injectedCall {
			x := *(*uintptr)(unsafe.Pointer(frame.sp))
			frame.sp += sys.MinFrameSize
			if GOARCH == "arm64" {
				// arm64 needs 16-byte aligned SP, always
				frame.sp += sys.PtrSize
			}
			f = findfunc(frame.pc)
			frame.fn = f
			if !f.valid() {
				frame.pc = x
			} else if funcspdelta(f, frame.pc, &cache) == 0 {
				frame.lr = x
			}
		}
	}

	if printing {
		n = nprint
	}

	// Note that panic != nil is okay here: there can be leftover panics,
	// because the defers on the panic stack do not nest in frame order as
	// they do on the defer stack. If you have:
	//
	//	frame 1 defers d1
	//	frame 2 defers d2
	//	frame 3 defers d3
	//	frame 4 panics
	//	frame 4's panic starts running defers
	//	frame 5, running d3, defers d4
	//	frame 5 panics
	//	frame 5's panic starts running defers
	//	frame 6, running d4, garbage collects
	//	frame 6, running d2, garbage collects
	//
	// During the execution of d4, the panic stack is d4 -> d3, which
	// is nested properly, and we'll treat frame 3 as resumable, because we
	// can find d3. (And in fact frame 3 is resumable. If d4 recovers
	// and frame 5 continues running, d3, d3 can recover and we'll
	// resume execution in (returning from) frame 3.)
	//
	// During the execution of d2, however, the panic stack is d2 -> d3,
	// which is inverted. The scan will match d2 to frame 2 but having
	// d2 on the stack until then means it will not match d3 to frame 3.
	// This is okay: if we're running d2, then all the defers after d2 have
	// completed and their corresponding frames are dead. Not finding d3
	// for frame 3 means we'll set frame 3's continpc == 0, which is correct
	// (frame 3 is dead). At the end of the walk the panic stack can thus
	// contain defers (d3 in this case) for dead frames. The inversion here
	// always indicates a dead frame, and the effect of the inversion on the
	// scan is to hide those dead frames, so the scan is still okay:
	// what's left on the panic stack are exactly (and only) the dead frames.
	//
	// We require callback != nil here because only when callback != nil
	// do we know that gentraceback is being called in a "must be correct"
	// context as opposed to a "best effort" context. The tracebacks with
	// callbacks only happen when everything is stopped nicely.
	// At other times, such as when gathering a stack for a profiling signal
	// or when printing a traceback during a crash, everything may not be
	// stopped nicely, and the stack walk may not be able to complete.
	if callback != nil && n < max && frame.sp != gp.stktopsp {
		print("runtime: g", gp.goid, ": frame.sp=", hex(frame.sp), " top=", hex(gp.stktopsp), "\n")
		print("\tstack=[", hex(gp.stack.lo), "-", hex(gp.stack.hi), "] n=", n, " max=", max, "\n")
		throw("traceback did not unwind completely")
	}

	return n
}

// reflectMethodValue is a partial duplicate of reflect.makeFuncImpl
// and reflect.methodValue.
type reflectMethodValue struct {
	fn     uintptr
	stack  *bitvector // ptrmap for both args and results
	argLen uintptr    // just args
}

// getArgInfoFast returns the argument frame information for a call to f.
// It is short and inlineable. However, it does not handle all functions.
// If ok reports false, you must call getArgInfo instead.
// TODO(josharian): once we do mid-stack inlining,
// call getArgInfo directly from getArgInfoFast and stop returning an ok bool.
func getArgInfoFast(f funcInfo, needArgMap bool) (arglen uintptr, argmap *bitvector, ok bool) {
	return uintptr(f.args), nil, !(needArgMap && f.args == _ArgsSizeUnknown)
}

// getArgInfo returns the argument frame information for a call to f
// with call frame frame.
//
// This is used for both actual calls with active stack frames and for
// deferred calls or goroutines that are not yet executing. If this is an actual
// call, ctxt must be nil (getArgInfo will retrieve what it needs from
// the active stack frame). If this is a deferred call or unstarted goroutine,
// ctxt must be the function object that was deferred or go'd.
func getArgInfo(frame *stkframe, f funcInfo, needArgMap bool, ctxt *funcval) (arglen uintptr, argmap *bitvector) {
	arglen = uintptr(f.args)
	if needArgMap && f.args == _ArgsSizeUnknown {
		// Extract argument bitmaps for reflect stubs from the calls they made to reflect.
		switch funcname(f) {
		case "reflect.makeFuncStub", "reflect.methodValueCall":
			// These take a *reflect.methodValue as their
			// context register.
			var mv *reflectMethodValue
			var retValid bool
			if ctxt != nil {
				// This is not an actual call, but a
				// deferred call or an unstarted goroutine.
				// The function value is itself the *reflect.methodValue.
				mv = (*reflectMethodValue)(unsafe.Pointer(ctxt))
			} else {
				// This is a real call that took the
				// *reflect.methodValue as its context
				// register and immediately saved it
				// to 0(SP). Get the methodValue from
				// 0(SP).
				arg0 := frame.sp + sys.MinFrameSize
				mv = *(**reflectMethodValue)(unsafe.Pointer(arg0))
				// Figure out whether the return values are valid.
				// Reflect will update this value after it copies
				// in the return values.
				retValid = *(*bool)(unsafe.Pointer(arg0 + 3*sys.PtrSize))
			}
			if mv.fn != f.entry {
				print("runtime: confused by ", funcname(f), "\n")
				throw("reflect mismatch")
			}
			bv := mv.stack
			arglen = uintptr(bv.n * sys.PtrSize)
			if !retValid {
				arglen = uintptr(mv.argLen) &^ (sys.PtrSize - 1)
			}
			argmap = bv
		}
	}
	return
}

// tracebackCgoContext handles tracing back a cgo context value, from
// the context argument to setCgoTraceback, for the gentraceback
// function. It returns the new value of n.
func tracebackCgoContext(pcbuf *uintptr, printing bool, ctxt uintptr, n, max int) int {
	var cgoPCs [32]uintptr
	cgoContextPCs(ctxt, cgoPCs[:])
	var arg cgoSymbolizerArg
	anySymbolized := false
	for _, pc := range cgoPCs {
		if pc == 0 || n >= max {
			break
		}
		if pcbuf != nil {
			(*[1 << 20]uintptr)(unsafe.Pointer(pcbuf))[n] = pc
		}
		if printing {
			if cgoSymbolizer == nil {
				print("non-Go function at pc=", hex(pc), "\n")
			} else {
				c := printOneCgoTraceback(pc, max-n, &arg)
				n += c - 1 // +1 a few lines down
				anySymbolized = true
			}
		}
		n++
	}
	if anySymbolized {
		arg.pc = 0
		callCgoSymbolizer(&arg)
	}
	return n
}

func printcreatedby(gp *g) {
	// Show what created goroutine, except main goroutine (goid 1).
	pc := gp.gopc
	f := findfunc(pc)
	if f.valid() && showframe(f, gp, false, funcID_normal, funcID_normal) && gp.goid != 1 {
		printcreatedby1(f, pc)
	}
}

func printcreatedby1(f funcInfo, pc uintptr) {
	print("created by ", funcname(f), "\n")
	tracepc := pc // back up to CALL instruction for funcline.
	if pc > f.entry {
		tracepc -= sys.PCQuantum
	}
	file, line := funcline(f, tracepc)
	print("\t", file, ":", line)
	if pc > f.entry {
		print(" +", hex(pc-f.entry))
	}
	print("\n")
}

func traceback(pc, sp, lr uintptr, gp *g) {
	traceback1(pc, sp, lr, gp, 0)
}

// tracebacktrap is like traceback but expects that the PC and SP were obtained
// from a trap, not from gp->sched or gp->syscallpc/gp->syscallsp or getcallerpc/getcallersp.
// Because they are from a trap instead of from a saved pair,
// the initial PC must not be rewound to the previous instruction.
// (All the saved pairs record a PC that is a return address, so we
// rewind it into the CALL instruction.)
// If gp.m.libcall{g,pc,sp} information is available, it uses that information in preference to
// the pc/sp/lr passed in.
func tracebacktrap(pc, sp, lr uintptr, gp *g) {
	if gp.m.libcallsp != 0 {
		// We're in C code somewhere, traceback from the saved position.
		traceback1(gp.m.libcallpc, gp.m.libcallsp, 0, gp.m.libcallg.ptr(), 0)
		return
	}
	traceback1(pc, sp, lr, gp, _TraceTrap)
}

func traceback1(pc, sp, lr uintptr, gp *g, flags uint) {
	// If the goroutine is in cgo, and we have a cgo traceback, print that.
	if iscgo && gp.m != nil && gp.m.ncgo > 0 && gp.syscallsp != 0 && gp.m.cgoCallers != nil && gp.m.cgoCallers[0] != 0 {
		// Lock cgoCallers so that a signal handler won't
		// change it, copy the array, reset it, unlock it.
		// We are locked to the thread and are not running
		// concurrently with a signal handler.
		// We just have to stop a signal handler from interrupting
		// in the middle of our copy.
		atomic.Store(&gp.m.cgoCallersUse, 1)
		cgoCallers := *gp.m.cgoCallers
		gp.m.cgoCallers[0] = 0
		atomic.Store(&gp.m.cgoCallersUse, 0)

		printCgoTraceback(&cgoCallers)
	}

	var n int
	if readgstatus(gp)&^_Gscan == _Gsyscall {
		// Override registers if blocked in system call.
		pc = gp.syscallpc
		sp = gp.syscallsp
		flags &^= _TraceTrap
	}
	// Print traceback. By default, omits runtime frames.
	// If that means we print nothing at all, repeat forcing all frames printed.
	n = gentraceback(pc, sp, lr, gp, 0, nil, _TracebackMaxFrames, nil, nil, flags)
	if n == 0 && (flags&_TraceRuntimeFrames) == 0 {
		n = gentraceback(pc, sp, lr, gp, 0, nil, _TracebackMaxFrames, nil, nil, flags|_TraceRuntimeFrames)
	}
	if n == _TracebackMaxFrames {
		print("...additional frames elided...\n")
	}
	printcreatedby(gp)

	if gp.ancestors == nil {
		return
	}
	for _, ancestor := range *gp.ancestors {
		printAncestorTraceback(ancestor)
	}
}

// printAncestorTraceback prints the traceback of the given ancestor.
// TODO: Unify this with gentraceback and CallersFrames.
func printAncestorTraceback(ancestor ancestorInfo) {
	print("[originating from goroutine ", ancestor.goid, "]:\n")
	for fidx, pc := range ancestor.pcs {
		f := findfunc(pc) // f previously validated
		if showfuncinfo(f, fidx == 0, funcID_normal, funcID_normal) {
			printAncestorTracebackFuncInfo(f, pc)
		}
	}
	if len(ancestor.pcs) == _TracebackMaxFrames {
		print("...additional frames elided...\n")
	}
	// Show what created goroutine, except main goroutine (goid 1).
	f := findfunc(ancestor.gopc)
	if f.valid() && showfuncinfo(f, false, funcID_normal, funcID_normal) && ancestor.goid != 1 {
		printcreatedby1(f, ancestor.gopc)
	}
}

// printAncestorTraceback prints the given function info at a given pc
// within an ancestor traceback. The precision of this info is reduced
// due to only have access to the pcs at the time of the caller
// goroutine being created.
func printAncestorTracebackFuncInfo(f funcInfo, pc uintptr) {
	name := funcname(f)
	if inldata := funcdata(f, _FUNCDATA_InlTree); inldata != nil {
		inltree := (*[1 << 20]inlinedCall)(inldata)
		ix := pcdatavalue(f, _PCDATA_InlTreeIndex, pc, nil)
		if ix >= 0 {
			name = funcnameFromNameoff(f, inltree[ix].func_)
		}
	}
	file, line := funcline(f, pc)
	if name == "runtime.gopanic" {
		name = "panic"
	}
	print(name, "(...)\n")
	print("\t", file, ":", line)
	if pc > f.entry {
		print(" +", hex(pc-f.entry))
	}
	print("\n")
}

func callers(skip int, pcbuf []uintptr) int {
	sp := getcallersp()
	pc := getcallerpc()
	gp := getg()
	var n int
	systemstack(func() {
		n = gentraceback(pc, sp, 0, gp, skip, &pcbuf[0], len(pcbuf), nil, nil, 0)
	})
	return n
}

func gcallers(gp *g, skip int, pcbuf []uintptr) int {
	return gentraceback(^uintptr(0), ^uintptr(0), 0, gp, skip, &pcbuf[0], len(pcbuf), nil, nil, 0)
}

// showframe reports whether the frame with the given characteristics should
// be printed during a traceback.
func showframe(f funcInfo, gp *g, firstFrame bool, funcID, childID funcID) bool {
	g := getg()
	if g.m.throwing > 0 && gp != nil && (gp == g.m.curg || gp == g.m.caughtsig.ptr()) {
		return true
	}
	return showfuncinfo(f, firstFrame, funcID, childID)
}

// showfuncinfo reports whether a function with the given characteristics should
// be printed during a traceback.
func showfuncinfo(f funcInfo, firstFrame bool, funcID, childID funcID) bool {
	level, _, _ := gotraceback()
	if level > 1 {
		// Show all frames.
		return true
	}

	if !f.valid() {
		return false
	}

	if funcID == funcID_wrapper && elideWrapperCalling(childID) {
		return false
	}

	name := funcname(f)

	// Special case: always show runtime.gopanic frame
	// in the middle of a stack trace, so that we can
	// see the boundary between ordinary code and
	// panic-induced deferred code.
	// See golang.org/issue/5832.
	if name == "runtime.gopanic" && !firstFrame {
		return true
	}

	return contains(name, ".") && (!hasPrefix(name, "runtime.") || isExportedRuntime(name))
}

// isExportedRuntime reports whether name is an exported runtime function.
// It is only for runtime functions, so ASCII A-Z is fine.
func isExportedRuntime(name string) bool {
	const n = len("runtime.")
	return len(name) > n && name[:n] == "runtime." && 'A' <= name[n] && name[n] <= 'Z'
}

// elideWrapperCalling reports whether a wrapper function that called
// function id should be elided from stack traces.
func elideWrapperCalling(id funcID) bool {
	// If the wrapper called a panic function instead of the
	// wrapped function, we want to include it in stacks.
	return !(id == funcID_gopanic || id == funcID_sigpanic || id == funcID_panicwrap)
}

var gStatusStrings = [...]string{
	_Gidle:      "idle",
	_Grunnable:  "runnable",
	_Grunning:   "running",
	_Gsyscall:   "syscall",
	_Gwaiting:   "waiting",
	_Gdead:      "dead",
	_Gcopystack: "copystack",
	_Gpreempted: "preempted",
}

func goroutineheader(gp *g) {
	gpstatus := readgstatus(gp)

	isScan := gpstatus&_Gscan != 0
	gpstatus &^= _Gscan // drop the scan bit

	// Basic string status
	var status string
	if 0 <= gpstatus && gpstatus < uint32(len(gStatusStrings)) {
		status = gStatusStrings[gpstatus]
	} else {
		status = "???"
	}

	// Override.
	if gpstatus == _Gwaiting && gp.waitreason != waitReasonZero {
		status = gp.waitreason.String()
	}

	// approx time the G is blocked, in minutes
	var waitfor int64
	if (gpstatus == _Gwaiting || gpstatus == _Gsyscall) && gp.waitsince != 0 {
		waitfor = (nanotime() - gp.waitsince) / 60e9
	}
	print("goroutine ", gp.goid, " [", status)
	if isScan {
		print(" (scan)")
	}
	if waitfor >= 1 {
		print(", ", waitfor, " minutes")
	}
	if gp.lockedm != 0 {
		print(", locked to thread")
	}
	print("]:\n")
}

func tracebackothers(me *g) {
	level, _, _ := gotraceback()

	// Show the current goroutine first, if we haven't already.
	g := getg()
	gp := g.m.curg
	if gp != nil && gp != me {
		print("\n")
		goroutineheader(gp)
		traceback(^uintptr(0), ^uintptr(0), 0, gp)
	}

	lock(&allglock)
	for _, gp := range allgs {
		if gp == me || gp == g.m.curg || readgstatus(gp) == _Gdead || isSystemGoroutine(gp, false) && level < 2 {
			continue
		}
		print("\n")
		goroutineheader(gp)
		// Note: gp.m == g.m occurs when tracebackothers is
		// called from a signal handler initiated during a
		// systemstack call. The original G is still in the
		// running state, and we want to print its stack.
		if gp.m != g.m && readgstatus(gp)&^_Gscan == _Grunning {
			print("\tgoroutine running on other thread; stack unavailable\n")
			printcreatedby(gp)
		} else {
			traceback(^uintptr(0), ^uintptr(0), 0, gp)
		}
	}
	unlock(&allglock)
}

// tracebackHexdump hexdumps part of stk around frame.sp and frame.fp
// for debugging purposes. If the address bad is included in the
// hexdumped range, it will mark it as well.
func tracebackHexdump(stk stack, frame *stkframe, bad uintptr) {
	const expand = 32 * sys.PtrSize
	const maxExpand = 256 * sys.PtrSize
	// Start around frame.sp.
	lo, hi := frame.sp, frame.sp
	// Expand to include frame.fp.
	if frame.fp != 0 && frame.fp < lo {
		lo = frame.fp
	}
	if frame.fp != 0 && frame.fp > hi {
		hi = frame.fp
	}
	// Expand a bit more.
	lo, hi = lo-expand, hi+expand
	// But don't go too far from frame.sp.
	if lo < frame.sp-maxExpand {
		lo = frame.sp - maxExpand
	}
	if hi > frame.sp+maxExpand {
		hi = frame.sp + maxExpand
	}
	// And don't go outside the stack bounds.
	if lo < stk.lo {
		lo = stk.lo
	}
	if hi > stk.hi {
		hi = stk.hi
	}

	// Print the hex dump.
	print("stack: frame={sp:", hex(frame.sp), ", fp:", hex(frame.fp), "} stack=[", hex(stk.lo), ",", hex(stk.hi), ")\n")
	hexdumpWords(lo, hi, func(p uintptr) byte {
		switch p {
		case frame.fp:
			return '>'
		case frame.sp:
			return '<'
		case bad:
			return '!'
		}
		return 0
	})
}

// Does f mark the top of a goroutine stack?
func topofstack(f funcInfo, g0 bool) bool {
	return f.funcID == funcID_goexit ||
		f.funcID == funcID_mstart ||
		f.funcID == funcID_mcall ||
		f.funcID == funcID_morestack ||
		f.funcID == funcID_rt0_go ||
		f.funcID == funcID_externalthreadhandler ||
		// asmcgocall is TOS on the system stack because it
		// switches to the system stack, but in this case we
		// can come back to the regular stack and still want
		// to be able to unwind through the call that appeared
		// on the regular stack.
		(g0 && f.funcID == funcID_asmcgocall)
}

// isSystemGoroutine reports whether the goroutine g must be omitted
// in stack dumps and deadlock detector. This is any goroutine that
// starts at a runtime.* entry point, except for runtime.main,
// runtime.handleAsyncEvent (wasm only) and sometimes runtime.runfinq.
//
// If fixed is true, any goroutine that can vary between user and
// system (that is, the finalizer goroutine) is considered a user
// goroutine.
func isSystemGoroutine(gp *g, fixed bool) bool {
	// Keep this in sync with cmd/trace/trace.go:isSystemGoroutine.
	f := findfunc(gp.startpc)
	if !f.valid() {
		return false
	}
	if f.funcID == funcID_runtime_main || f.funcID == funcID_handleAsyncEvent {
		return false
	}
	if f.funcID == funcID_runfinq {
		// We include the finalizer goroutine if it's calling
		// back into user code.
		if fixed {
			// This goroutine can vary. In fixed mode,
			// always consider it a user goroutine.
			return false
		}
		return !fingRunning
	}
	return hasPrefix(funcname(f), "runtime.")
}

// SetCgoTraceback 记录了三个 C 函数，用于从C代码中收集回溯信息，
// 并将回溯信息转换成符号信息。这些在打印使用cgo的程序的堆栈跟踪时使用。
//
// 追溯和上下文函数可以从信号处理程序中调用，因此必须只使用异步信号安全函数。
// 程序崩溃时可能会调用符号器函数，因此必须小心使用内存。没有一个函数可以调用 Go。
// 
// 上下文函数将用一个参数调用，一个指向结构的指针:
//
//	struct {
//		Context uintptr
//	}
//
// 在 C 语法中, 这个结构体将是
//
//	struct {
//		uintptr_t Context;
//	};
//
// 如果 Context 字段为0，则调用 context 函数来记录当前的回溯上下文。
// 它应该在 Context 字段中记录关于当前执行点所需的任何信息，以便以后生成堆栈跟踪，
// 可能是堆栈指针和 PC。在这种情况下，context 函数将从C代码中调用。
//
// 如果 Context 字段不是0，那么它是由先前对上下文函数的调用返回的值。
// 当不再需要上下文时调用这种情况；也就是说，当 Go 代码返回到它的C代码调用者时。
// 这允许 context 函数释放任何相关的资源。
//
// 虽然 context 函数在被调用时记录完整的堆栈跟踪是正确的，并且只需在回溯函数中复制出来，
// 但在典型的程序中，context 函数将被调用多次，而不会记录该上下文的回溯。
// 在对 context 函数的调用中记录完整的堆栈跟踪可能是低效的。
//
// 回溯函数将用一个参数调用，一个指向结构的指针:
//
//	struct {
//		Context    uintptr
//		SigContext uintptr
//		Buf        *uintptr
//		Max        uintptr
//	}
//
// 在 C 语法中, 这个结构体将是
//
//	struct {
//		uintptr_t  Context;
//		uintptr_t  SigContext;
//		uintptr_t* Buf;
//		uintptr_t  Max;
//	};
//
// Context 字段将为零，以收集从当前程序执行点的追溯。
// 在这种情况下，函数将从C代码中调用。
//
// 否则，Context 将是先前通过调用 context 函数返回的值。
// 回溯函数应该从程序执行中保存的点收集堆栈跟踪。
// 可以从记录上下文的执行线程之外的执行线程调用回溯函数，
// 但是仅当已知上下文有效且不变时。
// 回溯函数也可以在记录上下文的同一个线程的调用堆栈中更深的位置调用。
// 回溯函数可以用相同的上下文值调用多次；
// 如果可能的话，在第一次为特定的上下文值调用结果时，缓存结果通常是合适的。
//
// 如果从 Unix 系统上的信号处理程序调用回溯函数，
// SigContext 将是传递给信号处理程序的信号上下文参数
// (一个 C ucontext_t* 强制转换为 uintptr_t)。
// 这可用于在信号出现的点开始跟踪。如果不是从信号处理程序调用追溯函数，
// SigContext 将为零。
//
// Buf 是应该存储回溯信息的地方。应该是 PC 值，比如 Buf[0] 是调用者的 PC，
// Buf[1] 是那个函数的调用者的 PC，等等。Max是要存储的最大条目数。
// 该函数应该存储一个零来指示堆栈的顶部，或者调用方在一个不同的堆栈上，大概是一个 Go 堆栈。
//
// 不像 runtime.Callers，返回的 PC 值，当传递给符号函数时，
// 应该返回调用指令的文件/行。不需要额外的减法，也不合适。
//
// 在所有平台上，当从 Go 到 C 再到 Go 的调用请求堆栈跟踪时，都会调用回溯函数。
// 在 linux/amd64、linux/ppc64le 和 freebsd/amd64 上，
// 当正在执行cgo调用的线程收到信号时，也会调用回溯函数。
// 回溯函数不应假设何时调用它，因为 Go 的未来版本可能会进行额外的调用。
//
// symbolizer 函数将通过一个参数调用，一个指向结构的指针:
//
//	struct {
//		PC      uintptr // 获取信息的程序计数器
//		File    *byte   // 文件名 (NUL terminated)
//		Lineno  uintptr // 行号
//		Func    *byte   // 函数名 (NUL terminated)
//		Entry   uintptr // 函数入口点
//		More    uintptr // 如果此 PC 有更多信息，设置非零值
//		Data    uintptr // 运行时未使用的，可用于函数的
//	}
//
// 在 C 语法中, 这个结构体将是
//
//	struct {
//		uintptr_t PC;
//		char*     File;
//		uintptr_t Lineno;
//		char*     Func;
//		uintptr_t Entry;
//		uintptr_t More;
//		uintptr_t Data;
//	};
//
// PC 字段将是通过调用追溯函数返回的值。
//
// 第一次调用该函数进行特定的回溯时，除PC之外的所有字段都将为 0。
// 如果可能，该函数应填写其他字段，如果没有信息，则将其设置为 0/nil。
// 数据字段可用于存储通话中的任何有用信息。如果此 PC 有更多信息，
// 则 More 字段应设置为非零值，否则为零。
// 如果 More 设置为非零，该函数将在同一 PC 上再次调用，
// 并可能返回不同的信息(这是为了与内联函数一起使用)。如果 More 为零，
// 将使用回溯中的下一个 PC 值调用该函数。
// 当回溯完成时，函数将在PC设置为零的情况下被再次调用；
// 这可以用来释放任何信息。每次调用都会将结构的字段设置为返回时的相同值，
// 除了当 More 字段为零时的 PC 字段。函数不能在调用之间保留结构指针的副本。
//
// 调用 SetCgoTraceback 时，版本参数是函数预期接收的结构的版本号。目前这必须为零。
//
// symbolizer 函数可能为零，在这种情况下，回溯函数的结果将显示为数字。
// 如果回溯函数为零，则永远不会调用符号化函数。上下文函数可能为零，
// 在这种情况下，只能在上下文字段设置为零的情况下调用回溯函数。
// 如果上下文函数为零，那么从 Go 到 C 再到 Go 的调用不会显示调用堆栈的 C 部分的回溯。
//
// SetCgoTraceback 应该只调用一次，最好从init函数调用。
func SetCgoTraceback(version int, traceback, context, symbolizer unsafe.Pointer) {
	if version != 0 {
		panic("unsupported version")
	}

	if cgoTraceback != nil && cgoTraceback != traceback ||
		cgoContext != nil && cgoContext != context ||
		cgoSymbolizer != nil && cgoSymbolizer != symbolizer {
		panic("call SetCgoTraceback only once")
	}

	cgoTraceback = traceback
	cgoContext = context
	cgoSymbolizer = symbolizer

	// The context function is called when a C function calls a Go
	// function. As such it is only called by C code in runtime/cgo.
	if _cgo_set_context_function != nil {
		cgocall(_cgo_set_context_function, context)
	}
}

var cgoTraceback unsafe.Pointer
var cgoContext unsafe.Pointer
var cgoSymbolizer unsafe.Pointer

// cgoTracebackArg is the type passed to cgoTraceback.
type cgoTracebackArg struct {
	context    uintptr
	sigContext uintptr
	buf        *uintptr
	max        uintptr
}

// cgoContextArg is the type passed to the context function.
type cgoContextArg struct {
	context uintptr
}

// cgoSymbolizerArg is the type passed to cgoSymbolizer.
type cgoSymbolizerArg struct {
	pc       uintptr
	file     *byte
	lineno   uintptr
	funcName *byte
	entry    uintptr
	more     uintptr
	data     uintptr
}

// cgoTraceback prints a traceback of callers.
func printCgoTraceback(callers *cgoCallers) {
	if cgoSymbolizer == nil {
		for _, c := range callers {
			if c == 0 {
				break
			}
			print("non-Go function at pc=", hex(c), "\n")
		}
		return
	}

	var arg cgoSymbolizerArg
	for _, c := range callers {
		if c == 0 {
			break
		}
		printOneCgoTraceback(c, 0x7fffffff, &arg)
	}
	arg.pc = 0
	callCgoSymbolizer(&arg)
}

// printOneCgoTraceback prints the traceback of a single cgo caller.
// This can print more than one line because of inlining.
// Returns the number of frames printed.
func printOneCgoTraceback(pc uintptr, max int, arg *cgoSymbolizerArg) int {
	c := 0
	arg.pc = pc
	for c <= max {
		callCgoSymbolizer(arg)
		if arg.funcName != nil {
			// Note that we don't print any argument
			// information here, not even parentheses.
			// The symbolizer must add that if appropriate.
			println(gostringnocopy(arg.funcName))
		} else {
			println("non-Go function")
		}
		print("\t")
		if arg.file != nil {
			print(gostringnocopy(arg.file), ":", arg.lineno, " ")
		}
		print("pc=", hex(pc), "\n")
		c++
		if arg.more == 0 {
			break
		}
	}
	return c
}

// callCgoSymbolizer calls the cgoSymbolizer function.
func callCgoSymbolizer(arg *cgoSymbolizerArg) {
	call := cgocall
	if panicking > 0 || getg().m.curg != getg() {
		// We do not want to call into the scheduler when panicking
		// or when on the system stack.
		call = asmcgocall
	}
	if msanenabled {
		msanwrite(unsafe.Pointer(arg), unsafe.Sizeof(cgoSymbolizerArg{}))
	}
	call(cgoSymbolizer, noescape(unsafe.Pointer(arg)))
}

// cgoContextPCs gets the PC values from a cgo traceback.
func cgoContextPCs(ctxt uintptr, buf []uintptr) {
	if cgoTraceback == nil {
		return
	}
	call := cgocall
	if panicking > 0 || getg().m.curg != getg() {
		// We do not want to call into the scheduler when panicking
		// or when on the system stack.
		call = asmcgocall
	}
	arg := cgoTracebackArg{
		context: ctxt,
		buf:     (*uintptr)(noescape(unsafe.Pointer(&buf[0]))),
		max:     uintptr(len(buf)),
	}
	if msanenabled {
		msanwrite(unsafe.Pointer(&arg), unsafe.Sizeof(arg))
	}
	call(cgoTraceback, noescape(unsafe.Pointer(&arg)))
}
