// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sort_test

import (
	"fmt"
	"sort"
)

// Change 是一个源码变化记录，记录用户，语言和增量(delta size)。
type Change struct {
	user     string
	language string
	lines    int
}

type lessFunc func(p1, p2 *Change) bool

// multiSorter 实现了 Sort 接口，对其中的 changes 进行排序。
type multiSorter struct {
	changes []Change
	less    []lessFunc
}

// Sort 根据传递给 OrderedBy 的 less 函数对参数切片进行排序。
func (ms *multiSorter) Sort(changes []Change) {
	ms.changes = changes
	sort.Sort(ms)
}

// OrderedBy 返回使用 less 函数排序的 Sorter，按照顺序。
// 调用它的 Sort 方法去排序数据。
func OrderedBy(less ...lessFunc) *multiSorter {
	return &multiSorter{
		less: less,
	}
}

// Len 是 sort.Interface 的一部分。
func (ms *multiSorter) Len() int {
	return len(ms.changes)
}

// Swap 是 sort.Interface 的一部分。
func (ms *multiSorter) Swap(i, j int) {
	ms.changes[i], ms.changes[j] = ms.changes[j], ms.changes[i]
}

// Less 是 sort.Interface 的一部分。它是通过循环的方法实现的
// less 函数，直到它找到两个项（一个比另一个小）
// 的比较结果。注意，它每次调用
//  less 函数两次。我们可以让函数返回
// -1，0，1以减少调用的次数，以提高效率：
// 对读者的一个练习。
func (ms *multiSorter) Less(i, j int) bool {
	p, q := &ms.changes[i], &ms.changes[j]
	// 除了最后一个比较，其他的都试试。
	var k int
	for k = 0; k < len(ms.less)-1; k++ {
		less := ms.less[k]
		switch {
		case less(p, q):
			// p < q, 我们得到结果。
			return true
		case less(q, p):
			// p > q, 我们得到结果。
			return false
		}
		// p == q; 试图进行下一次比较。
	}
	// 这里所有的比较都是 equal，因此仅返回
	// 最终的比较结果。
	return ms.less[k](p, q)
}

var changes = []Change{
	{"gri", "Go", 100},
	{"ken", "C", 150},
	{"glenda", "Go", 200},
	{"rsc", "Go", 200},
	{"r", "Go", 100},
	{"ken", "Go", 200},
	{"dmr", "C", 100},
	{"r", "C", 150},
	{"gri", "Smalltalk", 80},
}

// ExampleMultiKeys 演示了一种在比较中使用多个字段的
// 不同集合对结构类型进行排序的技术。
// 我们将 "Less" 函数连接在一起，每一次比较使用一个单一的域。
func Example_sortMultiKeys() {
	// 排序 Change 结构体的闭包。
	user := func(c1, c2 *Change) bool {
		return c1.user < c2.user
	}
	language := func(c1, c2 *Change) bool {
		return c1.language < c2.language
	}
	increasingLines := func(c1, c2 *Change) bool {
		return c1.lines < c2.lines
	}
	decreasingLines := func(c1, c2 *Change) bool {
		return c1.lines > c2.lines // Note: > orders downwards.
	}

	// 简单的使用：通过 user 排序。
	OrderedBy(user).Sort(changes)
	fmt.Println("By user:", changes)

	// 更多的例子。
	OrderedBy(user, increasingLines).Sort(changes)
	fmt.Println("By user,<lines:", changes)

	OrderedBy(user, decreasingLines).Sort(changes)
	fmt.Println("By user,>lines:", changes)

	OrderedBy(language, increasingLines).Sort(changes)
	fmt.Println("By language,<lines:", changes)

	OrderedBy(language, increasingLines, user).Sort(changes)
	fmt.Println("By language,<lines,user:", changes)

	// Output:
	// By user: [{dmr C 100} {glenda Go 200} {gri Go 100} {gri Smalltalk 80} {ken C 150} {ken Go 200} {r Go 100} {r C 150} {rsc Go 200}]
	// By user,<lines: [{dmr C 100} {glenda Go 200} {gri Smalltalk 80} {gri Go 100} {ken C 150} {ken Go 200} {r Go 100} {r C 150} {rsc Go 200}]
	// By user,>lines: [{dmr C 100} {glenda Go 200} {gri Go 100} {gri Smalltalk 80} {ken Go 200} {ken C 150} {r C 150} {r Go 100} {rsc Go 200}]
	// By language,<lines: [{dmr C 100} {ken C 150} {r C 150} {r Go 100} {gri Go 100} {ken Go 200} {glenda Go 200} {rsc Go 200} {gri Smalltalk 80}]
	// By language,<lines,user: [{dmr C 100} {ken C 150} {r C 150} {gri Go 100} {r Go 100} {glenda Go 200} {ken Go 200} {rsc Go 200} {gri Smalltalk 80}]

}
