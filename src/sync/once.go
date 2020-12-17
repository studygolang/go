// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
)

// Once 一个对象只会执行一次.
type Once struct {
	// done 表示是否已经执行过.
	done uint32
	m    Mutex
}

// 只有在首次支持 Once 的这个实例的时，才会去触发响应的函数
// 换种说法, 定义一个 var once Once
// 如果 once.Do(f) 被多次调用, 即使f每次都有不一样的值，只有第一次会取调用函数 f。
// 如果要调用不同的f，需要给不同的f分配不同的Once实例，
// 每个 Once 实例只会被调用一次.
//
// Do 只会在初始化时运行一次. 由于 f 是一个无参函数
// 因此可能需要通过函数的变量来传递参数，类似如下:
// 	config.once.Do(func() { config.init(filename) })
//
// 注意不要在Do的f中再次去调用Do，否则将造成死锁.
//
// 如果 f panic, Do 也会认为已经返回了
// 后续再对 Do 的调用不会去调用 f.
//
func (o *Once) Do(f func()) {
	// Note: Here is an incorrect implementation of Do:
	//
	//	if atomic.CompareAndSwapUint32(&o.done, 0, 1) {
	//		f()
	//	}
	//
	// Do guarantees that when it returns, f has finished.
	// This implementation would not implement that guarantee:
	// given two simultaneous calls, the winner of the cas would
	// call f, and the second would return immediately, without
	// waiting for the first's call to f to complete.
	// This is why the slow path falls back to a mutex, and why
	// the atomic.StoreUint32 must be delayed until after f returns.

	if atomic.LoadUint32(&o.done) == 0 {
		// Outlined slow-path to allow inlining of the fast-path.
		o.doSlow(f)
	}
}

func (o *Once) doSlow(f func()) {
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}
