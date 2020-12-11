// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !goexperiment.staticlockranking

package sync

import "unsafe"

//  notifyList 在 runtime/sema.go 的类似结构体. 大小和对齐方式必须一致.
type notifyList struct {
	wait   uint32
	notify uint32
	lock   uintptr // key field of the mutex
	head   unsafe.Pointer
	tail   unsafe.Pointer
}
