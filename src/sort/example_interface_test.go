// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sort_test

import (
	"fmt"
	"sort"
)

type Person struct {
	Name string
	Age  int
}

func (p Person) String() string {
	return fmt.Sprintf("%s: %d", p.Name, p.Age)
}

// ByAge 基于 []Person 的 Age 域实现了sort.Interface。
//
type ByAge []Person

func (a ByAge) Len() int           { return len(a) }
func (a ByAge) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByAge) Less(i, j int) bool { return a[i].Age < a[j].Age }

func Example() {
	people := []Person{
		{"Bob", 31},
		{"John", 42},
		{"Michael", 17},
		{"Jenny", 26},
	}

	fmt.Println(people)
	// 排序切片有两种方式。
	// 第一种是像 ByAge 一样为切片类型定义一系列方法，然后调用 sort.Sort。
	// 这个例子中就使用的是这种技术。
	sort.Sort(ByAge(people))
	fmt.Println(people)

	// 另一种方式是使用将自定义的 Less 函数与 sort.Slice 一起使用，
	// 该函数可以作为闭包被提供。
	// 在这种方式中不需要方法被提供。（如果方法存在，会被忽略。）
	// 这里用相反的顺序重新排序：
	// 比较闭包和 ByAge.Less 就可看出
	sort.Slice(people, func(i, j int) bool {
		return people[i].Age > people[j].Age
	})
	fmt.Println(people)

	// Output:
	// [Bob: 31 John: 42 Michael: 17 Jenny: 26]
	// [Michael: 17 Jenny: 26 Bob: 31 John: 42]
	// [John: 42 Bob: 31 Jenny: 26 Michael: 17]
}
