// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unicode

// U+0100 下每个码点的位掩码，用于快速查找。
const (
	pC     = 1 << iota // 控制字符。
	pP                 // 标点字符。
	pN                 // 数字。
	pS                 // 符号字符。
	pZ                 // 空格字符。
	pLu                // 大写字母。
	pLl                // 小写字母。
	pp                 // Go 定义的可打印字符。
	pg     = pp | pZ   // Unicode 定义的图形字符。
	pLo    = pLl | pLu // 不区分大小写的字母。
	pLmask = pLo
)

// GraphicRanges 根据 Unicode 定义了图形字符集。
var GraphicRanges = []*RangeTable{
	L, M, N, P, S, Zs,
}

// PrintRanges 根据 Go 语言定义了可打印字符集。
// ASCII 空格，U+0020 分开处理。
var PrintRanges = []*RangeTable{
	L, M, N, P, S,
}

// IsGraphic 报告是否根据 Unicode 将字符定义为图形，这些字符包括字母，标记，数字，标点，符号和空格，它们来自 L， M， N， P， S， Zs等类别.
func IsGraphic(r rune) bool {
	// 转换为 uint32 以避免对负数测试。
	// 索引转换为 uint8 避免范围检查。
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pg != 0
	}
	return In(r, GraphicRanges...)
}

// IsPrint 报告该字符是否被 Go 定义为可打印字符，这些字符包括字母，标记，数字，标点符号，符号以及空格，它们来自类别 L，M，N，P，S 和 ASCII空格。此分类与 IsGraphic 相同，除了空格字符是 ASCII 空格 U+0020。
func IsPrint(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pp != 0
	}
	return In(r, PrintRanges...)
}

// IsOneOf 报告码点 r 是否为 ranges 其中之一的成员。
// In 函数提供了更好的签名，应该优先于 IsOneOf 使用。
func IsOneOf(ranges []*RangeTable, r rune) bool {
	for _, inside := range ranges {
		if Is(inside, r) {
			return true
		}
	}
	return false
}

// In 报告码点 r 是否是ranges其中之一的成员。
func In(r rune, ranges ...*RangeTable) bool {
	for _, inside := range ranges {
		if Is(inside, r) {
			return true
		}
	}
	return false
}

// IsControl 报告该码点是否为控制字符。
// C (其他) Unicode 类包含更多码点，例如代理；使用 Is(C, r)来测试。
func IsControl(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pC != 0
	}
	// 所有控制字符 < MaxLatin1。
	return false
}

// IsLetter 报告码点是否是字母 （L类）。
func IsLetter(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&(pLmask) != 0
	}
	return isExcludingLatin(Letter, r)
}

// IsMark 报告码点是否是标记符号 （M类）。
func IsMark(r rune) bool {
	// Latin-1 中没有标记符号。
	return isExcludingLatin(Mark, r)
}

// IsNumber 报告码点是否是数字（N类）。
func IsNumber(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pN != 0
	}
	return isExcludingLatin(Number, r)
}

// IsPunct 报告码点是否是标点符号（P类）。
func IsPunct(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pP != 0
	}
	return Is(Punct, r)
}

// IsSpace 报告码点是否是 Unicode Space property 定义的空白字符，在 Latin-1 中空白字符有 '\t', '\n', '\v', '\f', '\r', ' ', U+0085 (NEL), U+00A0 (NBSP)。
// 空白字符的其他定义由Z类和属性 Pattern_White_Space 设置。
func IsSpace(r rune) bool {
	// 该属性和Z不同；特殊情况。
	if uint32(r) <= MaxLatin1 {
		switch r {
		case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0:
			return true
		}
		return false
	}
	return isExcludingLatin(White_Space, r)
}

// IsSymbol 报告码点是否是符号字符。
func IsSymbol(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pS != 0
	}
	return isExcludingLatin(Symbol, r)
}
