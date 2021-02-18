// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// 这个例子演示了使用 heap 接口构建优先级队列。
package heap_test

import (
	"container/heap"
	"fmt"
)

// Item 是我们在优先级队列中管理的元素。
type Item struct {
	value    string // 项（item）的值；任意的。
	priority int    // 队列中项的优先级。
	// 该索引是更新所需的，并由 heap.Interface 方法维护。
	index int // 堆中的索引。
}

// PriorityQueue 实现了 heap.Interface，并保存了 Item。
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// 我们希望 Pop 给我们最高而非最低的优先级，因此我们使用在这里使用大于。
	return pq[i].priority > pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // 避免内存泄漏
	item.index = -1 // 处于安全考虑
	*pq = old[0 : n-1]
	return item
}

// update 修改在队列中的 Item 的优先级和值。
func (pq *PriorityQueue) update(item *Item, value string, priority int) {
	item.value = value
	item.priority = priority
	heap.Fix(pq, item.index)
}

// 本示例创建一个包含某些项的 PriorityQueue，添加和操作一个项，
// 然后按优先级顺序删除这些项。
func Example_priorityQueue() {
	// 一些项和他们的优先级
	items := map[string]int{
		"banana": 3, "apple": 2, "pear": 4,
	}

	// 创建一个优先级队列，并把之前创建的项放入，
	// 建立优先级队列（堆）不变式。
	pq := make(PriorityQueue, len(items))
	i := 0
	for value, priority := range items {
		pq[i] = &Item{
			value:    value,
			priority: priority,
			index:    i,
		}
		i++
	}
	heap.Init(&pq)

	// 插入一个新项之后修改它的优先级。
	item := &Item{
		value:    "orange",
		priority: 1,
	}
	heap.Push(&pq, item)
	pq.update(item, item.value, 5)

	// 取出项，按照递减的优先级顺序。
	for pq.Len() > 0 {
		item := heap.Pop(&pq).(*Item)
		fmt.Printf("%.2d:%s ", item.priority, item.value)
	}
	// Output:
	// 05:orange 04:pear 03:banana 02:apple
}
