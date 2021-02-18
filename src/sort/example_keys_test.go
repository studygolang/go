// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sort_test

import (
	"fmt"
	"sort"
)

// 几个类型定义增强可读性。
type earthMass float64
type au float64

// Planet 定义了太阳系对象的属性。
type Planet struct {
	name     string
	mass     earthMass
	distance au
}

// By 是一个 "less" 函数，这个函数定义了 Planet 参数的顺序。
type By func(p1, p2 *Planet) bool

// Sort 是函数类型 By 的方法，该方法根据函数对参数切片进行排序。
func (by By) Sort(planets []Planet) {
	ps := &planetSorter{
		planets: planets,
		by:      by, // Sort 方法的接收者是那个定义排序顺序的函数（闭包）
	}
	sort.Sort(ps)
}

// planetSorter 加入一个 By 函数和一个待排序的 Planets 切片。
type planetSorter struct {
	planets []Planet
	by      func(p1, p2 *Planet) bool // Less 方法中使用的闭包。
}

// Len 是 sort.Interface 的一部分。
func (s *planetSorter) Len() int {
	return len(s.planets)
}

// Swap 是 sort.Interface 的一部分。
func (s *planetSorter) Swap(i, j int) {
	s.planets[i], s.planets[j] = s.planets[j], s.planets[i]
}

// Less 是 sort.Interface 的一部分。它通过在排序器中调用 “ by” 闭包来实现。
func (s *planetSorter) Less(i, j int) bool {
	return s.by(&s.planets[i], &s.planets[j])
}

var planets = []Planet{
	{"Mercury", 0.055, 0.4},
	{"Venus", 0.815, 0.7},
	{"Earth", 1.0, 1.0},
	{"Mars", 0.107, 1.5},
}

// ExampleSortKeys 演示了使用可编程排序标准对结构类型进行排序的技术。
func Example_sortKeys() {
	// 排序 Planet 的闭包。
	name := func(p1, p2 *Planet) bool {
		return p1.name < p2.name
	}
	mass := func(p1, p2 *Planet) bool {
		return p1.mass < p2.mass
	}
	distance := func(p1, p2 *Planet) bool {
		return p1.distance < p2.distance
	}
	decreasingDistance := func(p1, p2 *Planet) bool {
		return distance(p2, p1)
	}

	// 按各种标准对行星进行排序。
	By(name).Sort(planets)
	fmt.Println("By name:", planets)

	By(mass).Sort(planets)
	fmt.Println("By mass:", planets)

	By(distance).Sort(planets)
	fmt.Println("By distance:", planets)

	By(decreasingDistance).Sort(planets)
	fmt.Println("By decreasing distance:", planets)

	// Output: By name: [{Earth 1 1} {Mars 0.107 1.5} {Mercury 0.055 0.4} {Venus 0.815 0.7}]
	// By mass: [{Mercury 0.055 0.4} {Mars 0.107 1.5} {Venus 0.815 0.7} {Earth 1 1}]
	// By distance: [{Mercury 0.055 0.4} {Venus 0.815 0.7} {Earth 1 1} {Mars 0.107 1.5}]
	// By decreasing distance: [{Mars 0.107 1.5} {Earth 1 1} {Venus 0.815 0.7} {Mercury 0.055 0.4}]
}
