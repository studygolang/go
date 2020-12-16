// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
	"unsafe"
)

// Cond 实现了一个条件变量
// 是goroutines等待或宣布事件触发的集合
//
// 每个 Cond 都有一个关联的锁 L (一般是一个 *Mutex 或 *RWMutex),
// 当去调用Wait方法以及修改条件变量的时候必须持有锁
//
// Cond在第一次使用后就不能再复制
type Cond struct {
	noCopy noCopy

	// 使用Cond的时候必须持有这个锁
	L Locker

	notify  notifyList
	checker copyChecker
}

// NewCond 使用传入的l锁生成新的 Cond
func NewCond(l Locker) *Cond {
	return &Cond{L: l}
}

// Wait 解锁c.L 并暂停goroutine的执行。
// 等待被唤醒后恢复执行，并重新锁定 c.L
// 与其他系统不同，等待不能返回，除非被广播或信号唤醒。
//
// 因为第一次恢复的时候c.L并未锁定，调用者一般不能认为条件为真。
// 应该在循环中等待,并持续的判断调试是否为真
//
//    c.L.Lock()
//    for !condition() {
//        c.Wait()
//    }
//    ... make use of condition ...
//    c.L.Unlock()
//
func (c *Cond) Wait() {
	c.checker.check()
	t := runtime_notifyListAdd(&c.notify)
	c.L.Unlock()
	runtime_notifyListWait(&c.notify, t)
	c.L.Lock()
}

// Signal 如果有正在等待的goroutine，会唤醒一个.
//
// 调用该方法的时候不需要持有c.L这个锁
func (c *Cond) Signal() {
	c.checker.check()
	runtime_notifyListNotifyOne(&c.notify)
}

// Broadcast 唤醒所有等待Cond的goroutine.
//
// 调用该方法的时候不需要持有c.L这个锁
func (c *Cond) Broadcast() {
	c.checker.check()
	runtime_notifyListNotifyAll(&c.notify)
}

// copyChecker 检测是否被复制.
type copyChecker uintptr

func (c *copyChecker) check() {
	if uintptr(*c) != uintptr(unsafe.Pointer(c)) &&
		!atomic.CompareAndSwapUintptr((*uintptr)(c), 0, uintptr(unsafe.Pointer(c))) &&
		uintptr(*c) != uintptr(unsafe.Pointer(c)) {
		panic("sync.Cond is copied")
	}
}

// noCopy 用来嵌入到struct中使用
// 禁止第一次使用后再次被复制使用
//
// 详情参考 https://golang.org/issues/8005#issuecomment-190753527
type noCopy struct{}

// Lock  是`go vet`命令用来做检查用的，防止使用后再次复制.
func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
