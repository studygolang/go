// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// utf8 包实现了 UTF-8 文本编码的函数和常量，其中包括码点 (rune) 和 UTF-8 字节序列相互转换的函数。详情参见 https://en.wikipedia.org/wiki/UTF-8
package utf8

// 测试文件中验证了 RuneError == unicode.ReplacementChar 和 MaxRune == unicode.MaxRune 两种情况。
// 在本文档定义它们避免了 utf8 包依赖 unicode 包。

// 用于 UTF-8 编码的数字。
const (
	RuneError = '\uFFFD'     // 错误码点或 Unicode 占位符
	RuneSelf  = 0x80         // 值小于 RuneSelf(128) 的字符是单字节字符
	MaxRune   = '\U0010FFFF' // Unicode 码点的最大值。
	UTFMax    = 4            // UTF-8 字符的最大字节数
)

// 在此范围内的码点在 UTF-8 中是非法的。
const (
	surrogateMin = 0xD800
	surrogateMax = 0xDFFF
)

const (
	t1 = 0b00000000
	tx = 0b10000000
	t2 = 0b11000000
	t3 = 0b11100000
	t4 = 0b11110000
	t5 = 0b11111000

	maskx = 0b00111111
	mask2 = 0b00011111
	mask3 = 0b00001111
	mask4 = 0b00000111

	rune1Max = 1<<7 - 1
	rune2Max = 1<<11 - 1
	rune3Max = 1<<16 - 1

	// 后续字节取值范围。
	locb = 0b10000000
	hicb = 0b10111111

    // 给如下常量选择这些名称是为了下表有更好的对齐效果。
    // 高位表示 acceptRanges 的索引，若是 F 则表示字符是单字节字符。
    // 低位表示字符长度，或是单字节字符的状态。
	xx = 0xF1 // 非法值: 长度 1
	as = 0xF0 // ASCII值: 长度 1
	s1 = 0x02 // 索引 0, 长度 2
	s2 = 0x13 // 索引 1, 长度 3
	s3 = 0x03 // 索引 0, 长度 3
	s4 = 0x23 // 索引 2, 长度 3
	s5 = 0x34 // 索引 3, 长度 4
	s6 = 0x04 // 索引 0, 长度 4
	s7 = 0x44 // 索引 4, 长度 4
)

// first 是 UTF-8 字符中首字节的编码信息。
var first = [256]uint8{
	//   1   2   3   4   5   6   7   8   9   A   B   C   D   E   F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x00-0x0F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x10-0x1F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x20-0x2F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x30-0x3F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x40-0x4F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x50-0x5F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x60-0x6F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x70-0x7F
	//   1   2   3   4   5   6   7   8   9   A   B   C   D   E   F
	xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, // 0x80-0x8F
	xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, // 0x90-0x9F
	xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, // 0xA0-0xAF
	xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, // 0xB0-0xBF
	xx, xx, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, // 0xC0-0xCF
	s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, // 0xD0-0xDF
	s2, s3, s3, s3, s3, s3, s3, s3, s3, s3, s3, s3, s3, s4, s3, s3, // 0xE0-0xEF
	s5, s6, s6, s6, s7, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, // 0xF0-0xFF
}

// acceptRange 给出 UTF-8 字符中第二个字节的有效值范围。
type acceptRange struct {
	lo uint8 // 第二个字节的最小值。
	hi uint8 // 第二个字节的最大值。
}

// acceptRanges 的大小为16，可避免代码进行边界检查。
var acceptRanges = [16]acceptRange{
	0: {locb, hicb},
	1: {0xA0, hicb},
	2: {locb, 0x9F},
	3: {0x90, hicb},
	4: {locb, 0x8F},
}

// FullRune 报告 p 是否以完整的 UTF-8 字符开头。
// 无效的编码也被认为是完整字符，因为它将被转换为一个宽度为1的 error rune。
func FullRune(p []byte) bool {
	n := len(p)
	if n == 0 {
		return false
	}
	x := first[p[0]]
	if n >= int(x&7) {
		return true // ASCII, invalid or valid.
	}
	// Must be short or invalid.
	accept := acceptRanges[x>>4]
	if n > 1 && (p[1] < accept.lo || accept.hi < p[1]) {
		return true
	} else if n > 2 && (p[2] < locb || hicb < p[2]) {
		return true
	}
	return false
}

// FullRuneInString 类似于 FullRune，但其输入为字符串。
func FullRuneInString(s string) bool {
	n := len(s)
	if n == 0 {
		return false
	}
	x := first[s[0]]
	if n >= int(x&7) {
		return true // ASCII, invalid, or valid.
	}
	// Must be short or invalid.
	accept := acceptRanges[x>>4]
	if n > 1 && (s[1] < accept.lo || accept.hi < s[1]) {
		return true
	} else if n > 2 && (s[2] < locb || hicb < s[2]) {
		return true
	}
	return false
}

// DecodeRune 解码 UTF-8 序列 p 中的第一个 Unicode 字符并返回字符码点及其宽度（以字节为单位）。如果 p 为空，则返回（RuneError，0）。若编码非法，则返回（RuneError，1）。对于正确的非空 UTF-8 字符，不可能返回这种结果。
//
// 如果不是正确的 UTF-8 编码，则它是非法的字符，比如对超出范围的值编码或是其值不是最短的 UTF-8 编码表示。非法字符不会执行任何验证。
func DecodeRune(p []byte) (r rune, size int) {
	n := len(p)
	if n < 1 {
		return RuneError, 0
	}
	p0 := p[0]
	x := first[p0]
	if x >= as {
		// 以下代码模拟对 x == xx 的附加检查，并相应地处理 ASCII 值和无效值的情况。这种 mask-and-or 的方法防止了额外的代码分支。
		mask := rune(x) << 31 >> 31 // Create 0x0000 or 0xFFFF.
		return rune(p[0])&^mask | RuneError&mask, 1
	}
	sz := int(x & 7)
	accept := acceptRanges[x>>4]
	if n < sz {
		return RuneError, 1
	}
	b1 := p[1]
	if b1 < accept.lo || accept.hi < b1 {
		return RuneError, 1
	}
	if sz <= 2 { // 用 <= 而非 == 来帮助编译器去除一些边界检查。
		return rune(p0&mask2)<<6 | rune(b1&maskx), 2
	}
	b2 := p[2]
	if b2 < locb || hicb < b2 {
		return RuneError, 1
	}
	if sz <= 3 {
		return rune(p0&mask3)<<12 | rune(b1&maskx)<<6 | rune(b2&maskx), 3
	}
	b3 := p[3]
	if b3 < locb || hicb < b3 {
		return RuneError, 1
	}
	return rune(p0&mask4)<<18 | rune(b1&maskx)<<12 | rune(b2&maskx)<<6 | rune(b3&maskx), 4
}

// DecodeRuneInString 类似于 DecodeRune，只是其输入为字符串。若 s 为空，则返回（RuneError，0）。若编码无效，则返回（RuneError，1）。对于正确的非空 UTF-8 字符，不可能返回这种结果。 
//
// 如果不是正确的 UTF-8 编码，则它是非法的字符，比如对超出范围的值编码或是其值不是最短的 UTF-8 编码表示。非法字符不会执行任何验证。
func DecodeRuneInString(s string) (r rune, size int) {
	n := len(s)
	if n < 1 {
		return RuneError, 0
	}
	s0 := s[0]
	x := first[s0]
	if x >= as {
		// 以下代码模拟对 x == xx 的附加检查，并相应地处理 ASCII 值和无效值的情况。这种 mask-and-or 的方法防止了额外的代码分支。
		mask := rune(x) << 31 >> 31 // Create 0x0000 or 0xFFFF.
		return rune(s[0])&^mask | RuneError&mask, 1
	}
	sz := int(x & 7)
	accept := acceptRanges[x>>4]
	if n < sz {
		return RuneError, 1
	}
	s1 := s[1]
	if s1 < accept.lo || accept.hi < s1 {
		return RuneError, 1
	}
	if sz <= 2 { // 用 <= 而非 == 来帮助编译器去除一些边界检查。
		return rune(s0&mask2)<<6 | rune(s1&maskx), 2
	}
	s2 := s[2]
	if s2 < locb || hicb < s2 {
		return RuneError, 1
	}
	if sz <= 3 {
		return rune(s0&mask3)<<12 | rune(s1&maskx)<<6 | rune(s2&maskx), 3
	}
	s3 := s[3]
	if s3 < locb || hicb < s3 {
		return RuneError, 1
	}
	return rune(s0&mask4)<<18 | rune(s1&maskx)<<12 | rune(s2&maskx)<<6 | rune(s3&maskx), 4
}

// DecodeLastRune 解码 UTF-8 序列 p 中的最后一个 Unicode 字符并返回字符码点及其宽度（以字节为单位）。如果 p 为空，则返回（RuneError，0）。若编码非法，则返回（RuneError，1）。对于正确的非空 UTF-8 字符，不可能返回这种结果。
//
// 如果不是正确的 UTF-8 编码，则它是非法的字符，比如对超出范围的值编码或是其值不是最短的 UTF-8 编码表示。非法字符不会执行任何验证。
func DecodeLastRune(p []byte) (r rune, size int) {
	end := len(p)
	if end == 0 {
		return RuneError, 0
	}
	start := end - 1
	r = rune(p[start])
	if r < RuneSelf {
		return r, 1
	}
	// 在向后遍历非法 UTF-8 字符时防止出现 O(n^2) 复杂度的循环。
	lim := end - UTFMax
	if lim < 0 {
		lim = 0
	}
	for start--; start >= lim; start-- {
		if RuneStart(p[start]) {
			break
		}
	}
	if start < 0 {
		start = 0
	}
	r, size = DecodeRune(p[start:end])
	if start+size != end {
		return RuneError, 1
	}
	return r, size
}

// DecodeLastRuneInString 类似于 DecodeLastRune，但其输入为字符串。如果 s 为空，则返回（RuneError，0）。若编码非法，则返回（RuneError，1）。对于正确的非空 UTF-8 字符，不可能返回这种结果。
//
// 如果不是正确的 UTF-8 编码，则它是非法的字符，比如对超出范围的值编码或是其值不是最短的 UTF-8 编码表示。非法字符不会执行任何验证。
func DecodeLastRuneInString(s string) (r rune, size int) {
	end := len(s)
	if end == 0 {
		return RuneError, 0
	}
	start := end - 1
	r = rune(s[start])
	if r < RuneSelf {
		return r, 1
	}
	// 在向后遍历非法 UTF-8 字符时防止出现 O(n^2) 复杂度的循环。
	lim := end - UTFMax
	if lim < 0 {
		lim = 0
	}
	for start--; start >= lim; start-- {
		if RuneStart(s[start]) {
			break
		}
	}
	if start < 0 {
		start = 0
	}
	r, size = DecodeRuneInString(s[start:end])
	if start+size != end {
		return RuneError, 1
	}
	return r, size
}

// RuneLen 返回编码码点时所需的字节数。
// 若该码点在 UTF-8 中属于非法值，则返回 -1。
func RuneLen(r rune) int {
	switch {
	case r < 0:
		return -1
	case r <= rune1Max:
		return 1
	case r <= rune2Max:
		return 2
	case surrogateMin <= r && r <= surrogateMax:
		return -1
	case r <= rune3Max:
		return 3
	case r <= MaxRune:
		return 4
	}
	return -1
}

// EncodeRune 将码点的 UTF-8 编码写入 p（必须足够大），返回值为写入的字节数。
func EncodeRune(p []byte, r rune) int {
	// 负值是错误的，将其转为无符号数即可解决。
	switch i := uint32(r); {
	case i <= rune1Max:
		p[0] = byte(r)
		return 1
	case i <= rune2Max:
		_ = p[1] // 去除边界检查。
		p[0] = t2 | byte(r>>6)
		p[1] = tx | byte(r)&maskx
		return 2
	case i > MaxRune, surrogateMin <= i && i <= surrogateMax:
		r = RuneError
		fallthrough
	case i <= rune3Max:
		_ = p[2] // 去除边界检查。
		p[0] = t3 | byte(r>>12)
		p[1] = tx | byte(r>>6)&maskx
		p[2] = tx | byte(r)&maskx
		return 3
	default:
		_ = p[3] // 去除边界检查。
		p[0] = t4 | byte(r>>18)
		p[1] = tx | byte(r>>12)&maskx
		p[2] = tx | byte(r>>6)&maskx
		p[3] = tx | byte(r)&maskx
		return 4
	}
}

// RuneCount 返回 p 中的码点数，错误和短编码被视为宽度为1字节的码点。
func RuneCount(p []byte) int {
	np := len(p)
	var n int
	for i := 0; i < np; {
		n++
		c := p[i]
		if c < RuneSelf {
			// ASCII fast path
			i++
			continue
		}
		x := first[c]
		if x == xx {
			i++ // invalid.
			continue
		}
		size := int(x & 7)
		if i+size > np {
			i++ // Short or invalid.
			continue
		}
		accept := acceptRanges[x>>4]
		if c := p[i+1]; c < accept.lo || accept.hi < c {
			size = 1
		} else if size == 2 {
		} else if c := p[i+2]; c < locb || hicb < c {
			size = 1
		} else if size == 3 {
		} else if c := p[i+3]; c < locb || hicb < c {
			size = 1
		}
		i += size
	}
	return n
}

// RuneCountInString 类似于 RuneCount，但其输入为字符串。
func RuneCountInString(s string) (n int) {
	ns := len(s)
	for i := 0; i < ns; n++ {
		c := s[i]
		if c < RuneSelf {
			// ASCII fast path
			i++
			continue
		}
		x := first[c]
		if x == xx {
			i++ // invalid.
			continue
		}
		size := int(x & 7)
		if i+size > ns {
			i++ // Short or invalid.
			continue
		}
		accept := acceptRanges[x>>4]
		if c := s[i+1]; c < accept.lo || accept.hi < c {
			size = 1
		} else if size == 2 {
		} else if c := s[i+2]; c < locb || hicb < c {
			size = 1
		} else if size == 3 {
		} else if c := s[i+3]; c < locb || hicb < c {
			size = 1
		}
		i += size
	}
	return n
}

// RuneStart 报告字节 b 是否可能是已编码的非法码点首字节。第二个及后续字节始终将高两位设置为10。
func RuneStart(b byte) bool { return b&0xC0 != 0x80 }

// Valid 报告 p 是否完全由合法的 UTF-8 编码码点组成。
func Valid(p []byte) bool {
	// 快速路径，每次迭代都检查并跳过8字节ASCII字符。
	for len(p) >= 8 {
		// 结合两个32位负载，可将同一份代码用于32位和64位平台。
        // 编译器可以在许多平台上为 first32 和 second32 生成32位负载。
		// 详情请参阅 test/codegen/memcombine.go。
		first32 := uint32(p[0]) | uint32(p[1])<<8 | uint32(p[2])<<16 | uint32(p[3])<<24
		second32 := uint32(p[4]) | uint32(p[5])<<8 | uint32(p[6])<<16 | uint32(p[7])<<24
		if (first32|second32)&0x80808080 != 0 {
			// 发现非 ASCII 字节 (>= RuneSelf)。
			break
		}
		p = p[8:]
	}
	n := len(p)
	for i := 0; i < n; {
		pi := p[i]
		if pi < RuneSelf {
			i++
			continue
		}
		x := first[pi]
		if x == xx {
			return false // 起始字节非法。
		}
		size := int(x & 7)
		if i+size > n {
			return false // Short or invalid.
		}
		accept := acceptRanges[x>>4]
		if c := p[i+1]; c < accept.lo || accept.hi < c {
			return false
		} else if size == 2 {
		} else if c := p[i+2]; c < locb || hicb < c {
			return false
		} else if size == 3 {
		} else if c := p[i+3]; c < locb || hicb < c {
			return false
		}
		i += size
	}
	return true
}

// ValidString 报告 s 是否完全由合法的 UTF-8 编码符文组成。
func ValidString(s string) bool {
	// 快速路径，每次迭代都检查并跳过8字节ASCII字符。
	for len(s) >= 8 {
		// 结合两个32位负载，可将同一份代码用于32位和64位平台。
        // 编译器可以在许多平台上为 first32 和 second32 生成32位负载。
		// 详情请参阅 test/codegen/memcombine.go。
		first32 := uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 | uint32(s[3])<<24
		second32 := uint32(s[4]) | uint32(s[5])<<8 | uint32(s[6])<<16 | uint32(s[7])<<24
		if (first32|second32)&0x80808080 != 0 {
			// 发现非 ASCII 字节 (>= RuneSelf)。
			break
		}
		s = s[8:]
	}
	n := len(s)
	for i := 0; i < n; {
		si := s[i]
		if si < RuneSelf {
			i++
			continue
		}
		x := first[si]
		if x == xx {
			return false // 起始字节非法。
		}
		size := int(x & 7)
		if i+size > n {
			return false // Short or invalid.
		}
		accept := acceptRanges[x>>4]
		if c := s[i+1]; c < accept.lo || accept.hi < c {
			return false
		} else if size == 2 {
		} else if c := s[i+2]; c < locb || hicb < c {
			return false
		} else if size == 3 {
		} else if c := s[i+3]; c < locb || hicb < c {
			return false
		}
		i += size
	}
	return true
}

// ValidRune 报告 r 是否可以编码为合法的 UTF-8。
// 超出范围或代理一半的码点是非法的。
func ValidRune(r rune) bool {
	switch {
	case 0 <= r && r < surrogateMin:
		return true
	case surrogateMax < r && r <= MaxRune:
		return true
	}
	return false
}
