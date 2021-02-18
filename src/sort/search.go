// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// 这个文件实现了二分查找（binary search）。

package sort

// Search 使用二分查找查找并返回在 [0, n) 内满足 f(i) 为真的
// 最小索引 i，假设在[0,n)处，
// f(i) == true 意味着 f(i+1) == true。也就是说，Search 需要
// f 对于输入范围 [0, n) 的靠前部分（可能是空的）为假，
// 对剩下的(可能为空的)为真；Search 返回
// 第一个满足条件的索引。如果没有这样的索引，Search 返回 n。
// (注意 “not found” 返回值不是-1，例如，
// strings.Index)。
// Search 只在对在 [0,n）内的 i 调用 f(i)。
//
// Search 的一个常见用法是在一种有序的、可索引的数据结构中
// 找到 x 的值的索引 i，如数组或切片。
// 在这种情况下，参数 f （通常是一个闭包）捕获值
// 数据结构是如何被索引的
// 命令。
//
// 例如，给定一个按升序排序的切片数据，
// 调用 Search(len(data)， func(i int) bool {return data[i] >= 23})
// 返回最小索引i，这个 i 使 data[i] >= 23。如果调用者
// 要查找 23 是否在片中，
// 必须单独测试 data[i] == 23。
//
// 搜索按降序排序的数据将使用<=
// 运算符，而不是>=运算符。
//
// 为了完成上面的示例，下面的代码尝试在一个升序的整数切片 data 中
// 查找值 x:
//
//	x := 23
//	i := sort.Search(len(data), func(i int) bool { return data[i] >= x })
//	if i < len(data) && data[i] == x {
//		// x is present at data[i]
//	} else {
//		// x is not present in data,
//		// but i is the index where it would be inserted.
//	}
//
// 作为一个更古怪的例子，这个程序猜测你的数字：
//
//	func GuessingGame() {
//		var s string
//		fmt.Printf("Pick an integer from 0 to 100.\n")
//		answer := sort.Search(100, func(i int) bool {
//			fmt.Printf("Is your number <= %d? ", i)
//			fmt.Scanf("%s", &s)
//			return s != "" && s[0] == 'y'
//		})
//		fmt.Printf("Your number is %d.\n", answer)
//	}
//
func Search(n int, f func(int) bool) int {
	// 定义 f(-1) == false and f(n) == true.
	// 不变式: f(i-1) == false, f(j) == true.
	i, j := 0, n
	for i < j {
		h := int(uint(i+j) >> 1) // 计算 h 时避免溢出
		// i ≤ h < j
		if !f(h) {
			i = h + 1 // 维持 f(i-1) == false
		} else {
			j = h // 维持 f(j) == true
		}
	}
	// i == j, f(i-1) == false, and f(j) (= f(i)) == true  =>  answer is i.
	return i
}

// 对于一般情况的封装。

// SearchInts 在一个 int 类型的有序切片中搜索 x，
// 并返回被 Search 指定的索引。
// 如果 x 不存在时返回值是插入 x 的索引(可能是 len(a))。
// 这个切片必须是按升序排列的。
//
func SearchInts(a []int, x int) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// SearchFloat64s 在一个 float64 类型的有序切片中搜索 x，
// 并返回被 Search 指定的索引。
// 如果 x 不存在时返回值是插入 x 的索引(可能是 len(a))。
// 这个切片必须是按升序排列的。
//
func SearchFloat64s(a []float64, x float64) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// SearchStrings 在一个 string 类型的有序切片中搜索 x，
// 并返回被 Search 指定的索引。
// 如果 x 不存在时返回值是插入 x 的索引(可能是 len(a))。
// 这个切片必须是按升序排列的。
//
func SearchStrings(a []string, x string) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// Search 返回将 SearchInts 应用于接收者和 x 的结果。
func (p IntSlice) Search(x int) int { return SearchInts(p, x) }

// Search 返回将 SearchFloat64s 应用于接收者和 x 的结果。
func (p Float64Slice) Search(x float64) int { return SearchFloat64s(p, x) }

// Search 返回将 SearchStrings 应用于接收者和 x 的结果。
func (p StringSlice) Search(x string) int { return SearchStrings(p, x) }
