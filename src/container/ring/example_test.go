// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ring_test

import (
	"container/ring"
	"fmt"
)

func ExampleRing_Len() {
	// 创建一个大小为 4 的环
	r := ring.New(4)

	// 打印出它的长度
	fmt.Println(r.Len())

	// Output:
	// 4
}

func ExampleRing_Next() {
	// 创建一个大小为 5 的环
	r := ring.New(5)

	// 得到环的长度
	n := r.Len()

	// 使用一些整数初始化环
	for i := 0; i < n; i++ {
		r.Value = i
		r = r.Next()
	}

	// 遍历环并打印它的内容
	for j := 0; j < n; j++ {
		fmt.Println(r.Value)
		r = r.Next()
	}

	// Output:
	// 0
	// 1
	// 2
	// 3
	// 4
}

func ExampleRing_Prev() {
	// 创建一个大小为5的环
	r := ring.New(5)

	// 得到环的长度
	n := r.Len()

	// 使用一些整数初始化环
	for i := 0; i < n; i++ {
		r.Value = i
		r = r.Next()
	}

	// 向后遍历环并打印它的内容
	for j := 0; j < n; j++ {
		r = r.Prev()
		fmt.Println(r.Value)
	}

	// Output:
	// 4
	// 3
	// 2
	// 1
	// 0
}

func ExampleRing_Do() {
	// 创建一个大小为5的环
	r := ring.New(5)

	// 得到环的长度
	n := r.Len()

	// 使用一些整数初始化环
	for i := 0; i < n; i++ {
		r.Value = i
		r = r.Next()
	}

	// 遍历环并打印它的内容
	r.Do(func(p interface{}) {
		fmt.Println(p.(int))
	})

	// Output:
	// 0
	// 1
	// 2
	// 3
	// 4
}

func ExampleRing_Move() {
	// 创建一个大小为5的环
	r := ring.New(5)

	// 得到环的长度
	n := r.Len()

	// 使用一些整数初始化环
	for i := 0; i < n; i++ {
		r.Value = i
		r = r.Next()
	}

	// 移动指针向前移动3步
	r = r.Move(3)

	// 遍历环并打印它的内容
	r.Do(func(p interface{}) {
		fmt.Println(p.(int))
	})

	// Output:
	// 3
	// 4
	// 0
	// 1
	// 2
}

func ExampleRing_Link() {
	// 创建两个环 r 和 s，大小都为2
	r := ring.New(2)
	s := ring.New(2)

	// 得到环的长度
	lr := r.Len()
	ls := s.Len()

	// 将环 r 初始化为 0
	for i := 0; i < lr; i++ {
		r.Value = 0
		r = r.Next()
	}

	// 将环 s 初始化为 1
	for j := 0; j < ls; j++ {
		s.Value = 1
		s = s.Next()
	}

	// 连接两个环
	rs := r.Link(s)

	// 遍历组合后的环并打印它的内容
	rs.Do(func(p interface{}) {
		fmt.Println(p.(int))
	})

	// Output:
	// 0
	// 0
	// 1
	// 1
}

func ExampleRing_Unlink() {
	// 创建一个大小为 6 的环
	r := ring.New(6)

	// 获得环的长度
	n := r.Len()

	// 使用一些整数初始化环
	for i := 0; i < n; i++ {
		r.Value = i
		r = r.Next()
	}

	// 从 r 中取消连接三个元素，开始于 r.Next()
	r.Unlink(3)

	// 遍历剩余的环并打印它的内容
	r.Do(func(p interface{}) {
		fmt.Println(p.(int))
	})

	// Output:
	// 0
	// 4
	// 5
}
