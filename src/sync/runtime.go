// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import "unsafe"

// 具体的细节请参考 runtime 包

// Semacquire 当等待到 *s > 0 时，开始去原子的递减.
// 它是给同步并发库使用的 sleep 原语，不应该直接调用该方法
func runtime_Semacquire(s *uint32)

// SemacquireMutex 跟 Semacquire 类似, 但是是用于互斥对象的竞争分析.
// 如果 lifo 为 true, waiter 会在 wait 队列的第一个位置.
// skipframes 是在跟踪期间需要忽略的帧数, 
// 统计从 runtime_SemacquireMutex's 的调用开始.
func runtime_SemacquireMutex(s *uint32, lifo bool, skipframes int)

// Semrelease 原子的增加 *s 并且去唤醒因为 Semacquire 而阻塞等待的 goroutine
// 它是一个底层的唤醒原语给同步机制使用的，不应该直接去调用
// 如果 handoff 为 true, 会直接将 count 给第一个等待的 waiter.
// skipframes 是在跟踪期间需要忽略的帧数，
// 统计从  runtime_Semrelease's  的调用开始.
func runtime_Semrelease(s *uint32, handoff bool, skipframes int)

// 去 runtime/sema.go 看详细的文档.
func runtime_notifyListAdd(l *notifyList) uint32

// 去 runtime/sema.go 看详细的文档.
func runtime_notifyListWait(l *notifyList, t uint32)

// 去 runtime/sema.go 看详细的文档.
func runtime_notifyListNotifyAll(l *notifyList)

// 去 runtime/sema.go 看详细的文档.
func runtime_notifyListNotifyOne(l *notifyList)

// 确保 sync 和 runtime 上的notifyList大小一致.
func runtime_notifyListCheck(size uintptr)
func init() {
	var n notifyList
	runtime_notifyListCheck(unsafe.Sizeof(n))
}

// runtime 提供的自旋功能.
// runtime_canSpin 代表此刻的自旋是否有意义.
func runtime_canSpin(i int) bool

// runtime_doSpin 开始自旋.
func runtime_doSpin()

func runtime_nanotime() int64
