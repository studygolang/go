// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// +build linux

package bytes_test

import (
	. "bytes"
	"syscall"
	"testing"
)

// 此文件测试检查数据非常接近页面边界的字节操作的情况。
// 我们希望确保这些操作不会跨界读取，
// 不会在它们不应该出现的地方导致页面错误。

// 这些测试只在 linux 上运行。测试的代码不受特定操作系统的限制。

// dangerousSlice 返回一个切片，它的前面和后面都是一个错误页面。
func dangerousSlice(t *testing.T) []byte {
	pagesize := syscall.Getpagesize()
	b, err := syscall.Mmap(0, 0, 3*pagesize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE)
	if err != nil {
		t.Fatalf("mmap failed %s", err)
	}
	err = syscall.Mprotect(b[:pagesize], syscall.PROT_NONE)
	if err != nil {
		t.Fatalf("mprotect low failed %s\n", err)
	}
	err = syscall.Mprotect(b[2*pagesize:], syscall.PROT_NONE)
	if err != nil {
		t.Fatalf("mprotect high failed %s\n", err)
	}
	return b[pagesize : 2*pagesize]
}

func TestEqualNearPageBoundary(t *testing.T) {
	t.Parallel()
	b := dangerousSlice(t)
	for i := range b {
		b[i] = 'A'
	}
	for i := 0; i <= len(b); i++ {
		Equal(b[:i], b[len(b)-i:])
		Equal(b[len(b)-i:], b[:i])
	}
}

func TestIndexByteNearPageBoundary(t *testing.T) {
	t.Parallel()
	b := dangerousSlice(t)
	for i := range b {
		idx := IndexByte(b[i:], 1)
		if idx != -1 {
			t.Fatalf("IndexByte(b[%d:])=%d, want -1\n", i, idx)
		}
	}
}

func TestIndexNearPageBoundary(t *testing.T) {
	t.Parallel()
	var q [64]byte
	b := dangerousSlice(t)
	if len(b) > 256 {
		// 只有快到一页的时候才会担心。
		b = b[len(b)-256:]
	}
	for j := 1; j < len(q); j++ {
		q[j-1] = 1 // 只在最后一个字节上发现差异
		for i := range b {
			idx := Index(b[i:], q[:j])
			if idx != -1 {
				t.Fatalf("Index(b[%d:], q[:%d])=%d, want -1\n", i, j, idx)
			}
		}
		q[j-1] = 0
	}
}
