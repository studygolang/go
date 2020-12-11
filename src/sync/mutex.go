// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// sync包提供了基本的同步原语，比如排它锁 
// 除了 Once 和 WaitGroup 类型, 
// 大部分的类型都是为底层并发使用的. 
// 在实际的业务中更推荐使用Channel来进行并发控制.
//
// 在此包中定义的所有类型的值都不能复制
package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

func throw(string) // provided by runtime

// Mutex 是一个互斥锁.
// Mutex 的零值是一个未锁定的Mutex.
//
// Mutex 在使用后，禁止复制.
type Mutex struct {
	state int32
	sema  uint32
}

// Locker 代表一个可以锁定和解锁的对象.
type Locker interface {
	Lock()
	Unlock()
}

const (
	mutexLocked = 1 << iota // mutex 锁定的值
	mutexWoken
	mutexStarving
	mutexWaiterShift = iota

	// Mutex fairness.
	//
	// Mutex 有两种模式: 正常模式和饥饿模式.
	// 正常模式下， waiter 在一个先进先出的的队列中, 
	// 但是被唤醒的goroutine不会直接拥有锁，需要和新来的goroutine进行竞争. 
	// 新来的goroutine优先级更高,因为他已经运行在CPU上了，所以唤醒的waiter大概率会输
	// 然后被唤醒的goroutine会被放到队列的最前面
	// 如果一个waiter超过1ms都没有获取到锁,
	// 那么mutex会进入到饥饿模式.
	//
	// 在饥饿模式下，队列中的goroutine直接获取到锁的拥有权
	// 新到的goroutine即使发现是未锁定的状态，也不会去获取锁,也不会去自旋. 
	// 然后它会加入到队列的尾部.
	//
	// 如果一个waiter持有锁后发现有一下两种情况之一
	// (1) 当前waiter是队列中的最后一个 (2) 他等待的时间少于1ms,
	// 那么会将mutex的模式改为正常模式.
	//
	// 正常模式下，可以拥有更好的性能，即使有等待锁的waiter，goroutine也可以连续多次获取锁.
	// 饥饿模式是对公平性和性能的平衡，可以避免过长的等待.
	starvationThresholdNs = 1e6
)

// Lock 锁定m.
// 如果互斥锁处于锁定状态, 那么会阻塞点当前的线程
// 知道这个互斥锁可以被锁定（即处于解锁状态）.
func (m *Mutex) Lock() {
	// 快速方式: mutex没有被锁定.
	if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
		if race.Enabled {
			race.Acquire(unsafe.Pointer(m))
		}
		return
	}
	// 这里为什么要创建fast path和slowpath，是为了建立内联来得到优化，原文如下：
	// Slow path (outlined so that the fast path can be inlined)
	m.lockSlow()
}

func (m *Mutex) lockSlow() {
	var waitStartTime int64
	starving := false
	awoke := false
	iter := 0
	old := m.state
	for {
		// Don't spin in starvation mode, ownership is handed off to waiters
		// so we won't be able to acquire the mutex anyway.
		if old&(mutexLocked|mutexStarving) == mutexLocked && runtime_canSpin(iter) {
			// Active spinning makes sense.
			// Try to set mutexWoken flag to inform Unlock
			// to not wake other blocked goroutines.
			if !awoke && old&mutexWoken == 0 && old>>mutexWaiterShift != 0 &&
				atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) {
				awoke = true
			}
			runtime_doSpin()
			iter++
			old = m.state
			continue
		}
		new := old
		// Don't try to acquire starving mutex, new arriving goroutines must queue.
		if old&mutexStarving == 0 {
			new |= mutexLocked
		}
		if old&(mutexLocked|mutexStarving) != 0 {
			new += 1 << mutexWaiterShift
		}
		// The current goroutine switches mutex to starvation mode.
		// But if the mutex is currently unlocked, don't do the switch.
		// Unlock expects that starving mutex has waiters, which will not
		// be true in this case.
		if starving && old&mutexLocked != 0 {
			new |= mutexStarving
		}
		if awoke {
			// The goroutine has been woken from sleep,
			// so we need to reset the flag in either case.
			if new&mutexWoken == 0 {
				throw("sync: inconsistent mutex state")
			}
			new &^= mutexWoken
		}
		if atomic.CompareAndSwapInt32(&m.state, old, new) {
			if old&(mutexLocked|mutexStarving) == 0 {
				break // locked the mutex with CAS
			}
			// If we were already waiting before, queue at the front of the queue.
			queueLifo := waitStartTime != 0
			if waitStartTime == 0 {
				waitStartTime = runtime_nanotime()
			}
			runtime_SemacquireMutex(&m.sema, queueLifo, 1)
			starving = starving || runtime_nanotime()-waitStartTime > starvationThresholdNs
			old = m.state
			if old&mutexStarving != 0 {
				// If this goroutine was woken and mutex is in starvation mode,
				// ownership was handed off to us but mutex is in somewhat
				// inconsistent state: mutexLocked is not set and we are still
				// accounted as waiter. Fix that.
				if old&(mutexLocked|mutexWoken) != 0 || old>>mutexWaiterShift == 0 {
					throw("sync: inconsistent mutex state")
				}
				delta := int32(mutexLocked - 1<<mutexWaiterShift)
				if !starving || old>>mutexWaiterShift == 1 {
					// Exit starvation mode.
					// Critical to do it here and consider wait time.
					// Starvation mode is so inefficient, that two goroutines
					// can go lock-step infinitely once they switch mutex
					// to starvation mode.
					delta -= mutexStarving
				}
				atomic.AddInt32(&m.state, delta)
				break
			}
			awoke = true
			iter = 0
		} else {
			old = m.state
		}
	}

	if race.Enabled {
		race.Acquire(unsafe.Pointer(m))
	}
}

// Unlock 释放锁.
// 如果去释放一个没有被加锁的mutex会造成run-time报错.
//
// Mutex 和goroutine没有绑定关系.
// 可以使用一个goroutine去锁定，然后用另外的goroutine去解锁 .
func (m *Mutex) Unlock() {
	if race.Enabled {
		_ = m.state
		race.Release(unsafe.Pointer(m))
	}

	// Fast path: drop lock bit.
	new := atomic.AddInt32(&m.state, -mutexLocked)
	if new != 0 {
		// Outlined slow path to allow inlining the fast path.
		// To hide unlockSlow during tracing we skip one extra frame when tracing GoUnblock.
		m.unlockSlow(new)
	}
}

func (m *Mutex) unlockSlow(new int32) {
	if (new+mutexLocked)&mutexLocked == 0 {
		throw("sync: unlock of unlocked mutex")
	}
	if new&mutexStarving == 0 {
		old := new
		for {
			// If there are no waiters or a goroutine has already
			// been woken or grabbed the lock, no need to wake anyone.
			// In starvation mode ownership is directly handed off from unlocking
			// goroutine to the next waiter. We are not part of this chain,
			// since we did not observe mutexStarving when we unlocked the mutex above.
			// So get off the way.
			if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken|mutexStarving) != 0 {
				return
			}
			// Grab the right to wake someone.
			new = (old - 1<<mutexWaiterShift) | mutexWoken
			if atomic.CompareAndSwapInt32(&m.state, old, new) {
				runtime_Semrelease(&m.sema, false, 1)
				return
			}
			old = m.state
		}
	} else {
		// Starving mode: handoff mutex ownership to the next waiter, and yield
		// our time slice so that the next waiter can start to run immediately.
		// Note: mutexLocked is not set, the waiter will set it after wakeup.
		// But mutex is still considered locked if mutexStarving is set,
		// so new coming goroutines won't acquire it.
		runtime_Semrelease(&m.sema, true, 1)
	}
}
