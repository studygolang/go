// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

// GOMAXPROCS 设置可以同时执行的最大 CPU 数，并返回之前的设置。
// 默认为 runtime.NumCPU 的值，如果 n<1，不改变当前设置。
// 当调度程序改进时，这个调用将消失。
func GOMAXPROCS(n int) int {
	if GOARCH == "wasm" && n > 1 {
		n = 1 // WebAssembly has no threads yet, so only one CPU is possible.
	}

	lock(&sched.lock)
	ret := int(gomaxprocs)
	unlock(&sched.lock)
	if n <= 0 || n == ret {
		return ret
	}

	stopTheWorldGC("GOMAXPROCS")

	// newprocs will be processed by startTheWorld
	newprocs = int32(n)

	startTheWorldGC()
	return ret
}

// NumCPU 返回当前进程可用的逻辑处理器的数量。
//
// 通过在进程启动时查询操作系统来检查可用 CPU 的集合。
// 进程启动后对操作系统 CPU 分配的更改不会得到反映。
func NumCPU() int {
	return int(ncpu)
}

// NumCgoCall 返回当前进程发出的 cgo 调用数。
func NumCgoCall() int64 {
	var n int64
	for mp := (*m)(atomic.Loadp(unsafe.Pointer(&allm))); mp != nil; mp = mp.alllink {
		n += int64(mp.ncgocall)
	}
	return n
}

// NumGoroutine 返回当前存在的 Goroutine 数。
func NumGoroutine() int {
	return int(gcount())
}

//go:linkname debug_modinfo runtime/debug.modinfo
func debug_modinfo() string {
	return modinfo
}
