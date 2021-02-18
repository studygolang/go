// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// heap 包提供了对任何实现 heap.Interface 接口的数据类型的操作。
// 堆（heap）是一个有特定属性的树，该树的每个结点是它所在子树的最小值节点（minimum-valued node）。
//
//
// 树中最小的元素是根，它的索引是 0。
//
// 堆是实现优先级队列的常用方法。 
// 构造一个优先级队列，需要实现 Heap 接口和指明优先级，
// Less 方法指明了优先级次序。
// Push 添加项，Pop 移除队列中优先级最高的项。
// 例子中包含了这样的实现； 文件 example_pq_test.go 有完整的源码。
//
package heap

import "sort"

// Interface 类型描述了一个使用这个包中例程的类型的需求。
// 任何实现 Interface 的类型可以被作为一个带有以下不变量最小堆使用
// （在调用 Init 后建立，或者在数据为空或已排序时建立）：
//
//	!h.Less(j, i) for 0 <= i < h.Len() and 2*i+1 <= j <= 2*i+2 and j < h.Len()
//
// 注意，这个接口中的 Push 和 Pop 是为了包中堆的实现。 
// 从堆中添加或删除使用 heap.Push 和 heap.Pop。
type Interface interface {
	sort.Interface
	Push(x interface{}) // 添加 x 作为元素的 Len()
	Pop() interface{}   // remove and return element Len() - 1.
}


// Init 建立此程序包中其他例程所需的堆不变式。
// Init 对于堆不变量是等幂的，并且可以在堆不变量无效时调用。
// n = h.Len() 处的复杂度是 O(log n)。
func Init(h Interface) {
	// heapify
	n := h.Len()
	for i := n/2 - 1; i >= 0; i-- {
		down(h, i, n)
	}
}

// Push 将元素 x 压入堆中。
// n = h.Len() 处的复杂度是 O(log n)。
func Push(h Interface, x interface{}) {
	h.Push(x)
	up(h, h.Len()-1)
}

// Pop 移除并返回堆中最小的元素（最小元素由 Less 决定）。
// n = h.Len() 处的复杂度是 O(log n)。
// Pop 等价于 Remove(h, 0)。
func Pop(h Interface) interface{} {
	n := h.Len() - 1
	h.Swap(0, n)
	down(h, 0, n)
	return h.Pop()
}

// Remove 移除并返回在堆中索引为 i 的元素。
// n = h.Len() 处的复杂度是 O(log n)。
func Remove(h Interface, i int) interface{} {
	n := h.Len() - 1
	if n != i {
		h.Swap(i, n)
		if !down(h, i, n) {
			up(h, i)
		}
	}
	return h.Pop()
}

// Fix 在索引为 i 的元素的值改变之后重建堆排序。
// 改变索引为 i 的元素的值，然后调用 Fix ，这与调用 Remove(h, j) 然后压入（Push）一个新值是等效的，
// 但是更高效。
// n = h.Len() 处的复杂度是 O(log n)。 
func Fix(h Interface, i int) {
	if !down(h, i, h.Len()) {
		up(h, i)
	}
}

func up(h Interface, j int) {
	for {
		i := (j - 1) / 2 // 父结点
		if i == j || !h.Less(j, i) {
			break
		}
		h.Swap(i, j)
		j = i
	}
}

func down(h Interface, i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // 在 int 溢出后 j1 < 0
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && h.Less(j2, j1) {
			j = j2 // = 2*i + 2  // 右孩子
		}
		if !h.Less(j, i) {
			break
		}
		h.Swap(i, j)
		i = j
	}
	return i > i0
}
