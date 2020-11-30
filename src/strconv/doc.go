// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// strconv包实现了基本数据类型和其字符串表示的相互转换。
//
// 数字转换
//
// 最常见的数值转换是 Atoi (string to int)和 Itoa (int to string)。
//
//	i, err := strconv.Atoi("-42")
//	s := strconv.Itoa(-42)
//
// 它们假定是十进制的 int 类型。
//
// ParseBool, ParseFloat, ParseInt, 和 ParseUint 将字符串转换为对应值:
//
//	b, err := strconv.ParseBool("true")
//	f, err := strconv.ParseFloat("3.1415", 64)
//	i, err := strconv.ParseInt("-42", 10, 64)
//	u, err := strconv.ParseUint("42", 10, 64)
//
// 函数返回其类型精度最大的类型（float64, int64 和 uint64），
// 但是如果 size 设置精度较小，结果可以无损转换为较小精度的值：
//
//	s := "2147483647" // biggest int32
//	i64, err := strconv.ParseInt(s, 10, 32)
//	...
//	i := int32(i64)
//
// FormatBool, FormatFloat, FormatInt, 和 FormatUint 将值转换为字符串:
//
//	s := strconv.FormatBool(true)
//	s := strconv.FormatFloat(3.1415, 'E', -1, 64)
//	s := strconv.FormatInt(-42, 16)
//	s := strconv.FormatUint(42, 16)
//
// AppendBool, AppendFloat, AppendInt, 和 AppendUint 与上述函数功能相同，但将格式化后的值附加到目标 slice中。
//
// 字符串转换
//
// Quote 和 QuoteToASCII 用于给字符串添加双引号。
// 后者通过 \u 转义任何非 ASCII 编码字符保证返回结果是一个 ASCII 字符串：
//
//	q := strconv.Quote("Hello, 世界")
//	q := strconv.QuoteToASCII("Hello, 世界")
//
// QuoteRune 和 QuoteRuneToASCII 与上述函数功能相同，但是接收的是 rune 类型，添加的是单引号。
//
// Unquote 和 UnquoteChar 取消 string 和 rune 的引号。
//
package strconv
