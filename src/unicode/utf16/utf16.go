// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// utf16 包实现了 UTF-16 序列编解码。
package utf16

// 在测试中验证了 replaceChar == unicode.ReplacementChar 和 maxRune == unicode.MaxRune 两种情形。在本文档定义它们可避免依赖 unicode 包。

const (
	replacementChar = '\uFFFD'     // Unicode 占位符
	maxRune         = '\U0010FFFF' // UTF-16 中最大码点值
)

const (
	// 0xd800-0xdc00 编码代理对的高10位。
	// 0xdc00-0xe000 编码代理对的低10位。
	// 值为20位加上0x10000.
	surr1 = 0xd800
	surr2 = 0xdc00
	surr3 = 0xe000

	surrSelf = 0x10000
)

// IsSurrogate 报告特定的 Unicode 码点是否可以出现在代理对中。
func IsSurrogate(r rune) bool {
	return surr1 <= r && r < surr3
}

// DecodeRune 返回代理对的 UTF-16 解码值。
// 如果该代理对不是合法的 UTF-16 代理对，则 DecodeRune 返回 Unicode 占位符 U+FFFD。
func DecodeRune(r1, r2 rune) rune {
	if surr1 <= r1 && r1 < surr2 && surr2 <= r2 && r2 < surr3 {
		return (r1-surr1)<<10 | (r2 - surr2) + surrSelf
	}
	return replacementChar
}

// EncodeRune 返回给定码点的 UTF-16 代理对 r1，r2。
// 如果该码点不是有效的 Unicode 码点或不需要编码，则 EncodeRune 返回 U+FFFD，U+FFFD。
func EncodeRune(r rune) (r1, r2 rune) {
	if r < surrSelf || r > maxRune {
		return replacementChar, replacementChar
	}
	r -= surrSelf
	return surr1 + (r>>10)&0x3ff, surr2 + r&0x3ff
}

// Encode 返回 Unicode 码点序列 s 的 UTF-16 编码。
func Encode(s []rune) []uint16 {
	n := len(s)
	for _, v := range s {
		if v >= surrSelf {
			n++
		}
	}

	a := make([]uint16, n)
	n = 0
	for _, v := range s {
		switch {
		case 0 <= v && v < surr1, surr3 <= v && v < surrSelf:
			// 普通码点。
			a[n] = uint16(v)
			n++
		case surrSelf <= v && v <= maxRune:
			// 需要代理序列。
			r1, r2 := EncodeRune(v)
			a[n] = uint16(r1)
			a[n+1] = uint16(r2)
			n += 2
		default:
			a[n] = uint16(replacementChar)
			n++
		}
	}
	return a[:n]
}

// Decode 返回以 UTF-16 编码的 s 的 Unicode 码点序列。
func Decode(s []uint16) []rune {
	a := make([]rune, len(s))
	n := 0
	for i := 0; i < len(s); i++ {
		switch r := s[i]; {
		case r < surr1, surr3 <= r:
			// 普通码点。
			a[n] = rune(r)
		case surr1 <= r && r < surr2 && i+1 < len(s) &&
			surr2 <= s[i+1] && s[i+1] < surr3:
			// 合法的代理序列。
			a[n] = DecodeRune(rune(r), rune(s[i+1]))
			i++
		default:
			// 非法的代理序列。
			a[n] = replacementChar
		}
		n++
	}
	return a[:n]
}
