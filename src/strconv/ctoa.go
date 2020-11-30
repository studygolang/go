// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strconv

// FormatComplex 根据 fmt 格式和精确度 prec 将复数 c 转换为 (a+bi) 的形式，其中 a 和 b 分别表示实部和虚部。
//
// fmt 格式和精确度 prec 与 FormatFloat 中的定义相同。
// 它假设原始数据是从 bitSize 位的复数值中获得的，复数64必须是64，复数128必须是128。
func FormatComplex(c complex128, fmt byte, prec, bitSize int) string {
	if bitSize != 64 && bitSize != 128 {
		panic("invalid bitSize")
	}
	bitSize >>= 1 // complex64 uses float32 internally

	// Check if imaginary part has a sign. If not, add one.
	im := FormatFloat(imag(c), fmt, prec, bitSize)
	if im[0] != '+' && im[0] != '-' {
		im = "+" + im
	}

	return "(" + FormatFloat(real(c), fmt, prec, bitSize) + im + "i)"
}
