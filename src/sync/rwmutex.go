// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

// 该文件有一个修改后的副本在 runtime/rwmutex.go.
// 如果你在这里做了任何的修改，应该检查是否需要修改上面提到的副本中的文件.

// RWMutex 是一个 读/写 互斥锁.
// 读写锁支持多个 reader 读取或者一个 writer 去写.
// RWMutex 的零值时一个未加锁的互斥锁.
//
// RWMutex 使用后禁止复制.
//
// 如果goroutine拥有 RWMutex 进行读取，并且另外一个 goroutine 调用 Lock
// 在释放初始化读取锁之前，任何 goroutine 都不会获取到读取锁
// 为了确保最终锁的可用，禁止递归调用读取锁.
type RWMutex struct {
	w           Mutex  // writer的互斥锁
	writerSem   uint32 // writer的信号量
	readerSem   uint32 // reader的信号量
	readerCount int32  // reader的数量
	readerWait  int32  // 进行write需要等待的reader数量
}

const rwmutexMaxReaders = 1 << 30

// RLock 读取锁锁定.
//
// 禁止递归调用. 阻塞中的 Lock 调用会阻止新的 reader 来获取锁.
func (rw *RWMutex) RLock() {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	if atomic.AddInt32(&rw.readerCount, 1) < 0 {
		// A writer is pending, wait for it.
		runtime_SemacquireMutex(&rw.readerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
	}
}

// RUnlock 解锁一个RLock锁定的锁;
// 它不会影响同时并发读的其他 goroutine.
// 如果去解锁一个没有锁定的锁，会导致报错.
func (rw *RWMutex) RUnlock() {
	if race.Enabled {
		_ = rw.w.state
		race.ReleaseMerge(unsafe.Pointer(&rw.writerSem))
		race.Disable()
	}
	if r := atomic.AddInt32(&rw.readerCount, -1); r < 0 {
		// Outlined slow-path to allow the fast-path to be inlined
		rw.rUnlockSlow(r)
	}
	if race.Enabled {
		race.Enable()
	}
}

func (rw *RWMutex) rUnlockSlow(r int32) {
	if r+1 == 0 || r+1 == -rwmutexMaxReaders {
		race.Enable()
		throw("sync: RUnlock of unlocked RWMutex")
	}
	// A writer is pending.
	if atomic.AddInt32(&rw.readerWait, -1) == 0 {
		// The last reader unblocks the writer.
		runtime_Semrelease(&rw.writerSem, false, 1)
	}
}

// Lock 锁定一个写锁.
// 如果已经被锁定读取或者写入,
// Lock 会阻塞直到锁可用.
func (rw *RWMutex) Lock() {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	// First, resolve competition with other writers.
	rw.w.Lock()
	// Announce to readers there is a pending writer.
	r := atomic.AddInt32(&rw.readerCount, -rwmutexMaxReaders) + rwmutexMaxReaders
	// Wait for active readers.
	if r != 0 && atomic.AddInt32(&rw.readerWait, r) != 0 {
		runtime_SemacquireMutex(&rw.writerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
		race.Acquire(unsafe.Pointer(&rw.writerSem))
	}
}

// Unlock 解锁一个写入锁.
// 如果解锁一个没有锁定的写入锁，会导致报错.
//
// 和 Mutex 一样，RWMutex 也是不与 goroutine 绑定的
// 可以是不同的 goroutine 分别进行加锁和解锁的操作.
func (rw *RWMutex) Unlock() {
	if race.Enabled {
		_ = rw.w.state
		race.Release(unsafe.Pointer(&rw.readerSem))
		race.Disable()
	}

	// Announce to readers there is no active writer.
	r := atomic.AddInt32(&rw.readerCount, rwmutexMaxReaders)
	if r >= rwmutexMaxReaders {
		race.Enable()
		throw("sync: Unlock of unlocked RWMutex")
	}
	// Unblock blocked readers, if any.
	for i := 0; i < int(r); i++ {
		runtime_Semrelease(&rw.readerSem, false, 0)
	}
	// Allow other writers to proceed.
	rw.w.Unlock()
	if race.Enabled {
		race.Enable()
	}
}

// RLocker 返回一个 Locker 接口的读锁的加解锁实现.
func (rw *RWMutex) RLocker() Locker {
	return (*rlocker)(rw)
}

type rlocker RWMutex

func (r *rlocker) Lock()   { (*RWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*RWMutex)(r).RUnlock() }
