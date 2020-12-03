// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

// WaitGroup 用来等待一组goroutines的结束，.
// 主goroutine 调用Add方法去设置一共需要等待的数量
// 每个被等待的goroutine在执行完毕的时候需要执行Done方法来表示完成
// 同时,调用Wait方法可以阻塞主goroutine直到所有的goroutine完成
//
// WaitGroup 在时候后不允许拷贝.
type WaitGroup struct {
	noCopy noCopy

	// 64-bit 的值分成两段: 高 32 bits 是用来计数的, 低 32 bits 是waiter的值.
	// 64-bit 的原子操作需要64bit对齐,
	// 但是32-bit不需要对齐.
	// 因此我们分配了12个byte，用其中对齐的8个byte存储状态，剩余的4个保存信号量
	state1 [3]uint32
}

// state returns pointers to the state and sema fields stored within wg.state1.
func (wg *WaitGroup) state() (statep *uint64, semap *uint32) {
	if uintptr(unsafe.Pointer(&wg.state1))%8 == 0 {
		return (*uint64)(unsafe.Pointer(&wg.state1)), &wg.state1[2]
	} else {
		return (*uint64)(unsafe.Pointer(&wg.state1[1])), &wg.state1[0]
	}
}

// Add 用来控制等待的所有的goroutine的数量，可以使用负数来减少等待的数量.
// 当 counter 为0时，那么Wait方法会继续执行.
// 如果 counter 是负数，Add会直接panic.
//
// 是调用Wait方式之前，必须先调用Add方法设置需要等待的数量，并且不能为负数
// 在counter大于0时，可以Add任意的值，正数或负数
// 通常来说调用Add方法应该在创建goroutine之前.
// 如果要重用WaitGroup，那么必须单独等待每一组全部完成后，Wait响应之后
// 重新调用Add方法.
func (wg *WaitGroup) Add(delta int) {
	statep, semap := wg.state()
	if race.Enabled {
		_ = *statep // trigger nil deref early
		if delta < 0 {
			// Synchronize decrements with Wait.
			race.ReleaseMerge(unsafe.Pointer(wg))
		}
		race.Disable()
		defer race.Enable()
	}
	state := atomic.AddUint64(statep, uint64(delta)<<32)
	v := int32(state >> 32)
	w := uint32(state)
	if race.Enabled && delta > 0 && v == int32(delta) {
		// The first increment must be synchronized with Wait.
		// Need to model this as a read, because there can be
		// several concurrent wg.counter transitions from 0.
		race.Read(unsafe.Pointer(semap))
	}
	if v < 0 {
		panic("sync: negative WaitGroup counter")
	}
	if w != 0 && delta > 0 && v == int32(delta) {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	if v > 0 || w == 0 {
		return
	}
	// This goroutine has set counter to 0 when waiters > 0.
	// Now there can't be concurrent mutations of state:
	// - Adds must not happen concurrently with Wait,
	// - Wait does not increment waiters if it sees counter == 0.
	// Still do a cheap sanity check to detect WaitGroup misuse.
	if *statep != state {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	// Reset waiters count to 0.
	*statep = 0
	for ; w != 0; w-- {
		runtime_Semrelease(semap, false, 0)
	}
}

// Done 减少1个等待的counter.
func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

// Wait 阻塞goroutine直到需要等待的goroutine数量为0
func (wg *WaitGroup) Wait() {
	statep, semap := wg.state()
	if race.Enabled {
		_ = *statep // trigger nil deref early
		race.Disable()
	}
	for {
		state := atomic.LoadUint64(statep)
		v := int32(state >> 32)
		w := uint32(state)
		if v == 0 {
			// Counter is 0, no need to wait.
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			return
		}
		// Increment waiters count.
		if atomic.CompareAndSwapUint64(statep, state, state+1) {
			if race.Enabled && w == 0 {
				// Wait must be synchronized with the first Add.
				// Need to model this is as a write to race with the read in Add.
				// As a consequence, can do the write only for the first waiter,
				// otherwise concurrent Waits will race with each other.
				race.Write(unsafe.Pointer(semap))
			}
			runtime_Semacquire(semap)
			if *statep != 0 {
				panic("sync: WaitGroup is reused before previous Wait has returned")
			}
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			return
		}
	}
}
