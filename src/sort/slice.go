// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sort

// Slice 根据提供的 less 函数对所提供的切片进行排序。
//
// 不能保证排序是稳定的。对于稳定排序，请使用
// SliceStable。
//
// 如果提供的接口不是一个切片，函数会出现 panic。
func Slice(slice interface{}, less func(i, j int) bool) {
	rv := reflectValueOf(slice)
	swap := reflectSwapper(slice)
	length := rv.Len()
	quickSort_func(lessSwap{less, swap}, 0, length, maxDepth(length))
}

// SliceStable 根据提供的 less 函数对所提供的切片进行排序，
// 同时使相等的元素保持原来的顺序。
//
// 如果提供的接口不是一个切片，函数会出现 panic。
func SliceStable(slice interface{}, less func(i, j int) bool) {
	rv := reflectValueOf(slice)
	swap := reflectSwapper(slice)
	stable_func(lessSwap{less, swap}, rv.Len())
}

// SliceIsSorted 测试切片是否已排序。
//
// 如果提供的接口不是一个片，函数会出现 panic。
func SliceIsSorted(slice interface{}, less func(i, j int) bool) bool {
	rv := reflectValueOf(slice)
	n := rv.Len()
	for i := n - 1; i > 0; i-- {
		if less(i, i-1) {
			return false
		}
	}
	return true
}
