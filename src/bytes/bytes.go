// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// bytes 包实现用于操作字节切片的功能。
// 它类似于 string 包的功能。
package bytes

import (
	"internal/bytealg"
	"unicode"
	"unicode/utf8"
)

// Equal 判断 a 和 b 是否长度相等且包含相同的字节。
// nil 参数相当于一个空切片。
func Equal(a, b []byte) bool {
	// cmd/compile 和 gccgo 都不分配这些字符串转换。
	return string(a) == string(b)
}

// Compare 返回一个将两个字节切片按字典比较的的整数。
// 如果 a == b，结果将为 0；如果a < b，结果将为 -1；如果a > b，结果将为 +1。
// nil 参数相当于一个空切片。
func Compare(a, b []byte) int {
	return bytealg.Compare(a, b)
}

// 将 s 分解为 UTF-8 序列的一个切片，每个 Unicode 代码点（仍然是字节切片），
// 最多 n 个字节切片。无效的 UTF-8 序列被切成单个字节。
func explode(s []byte, n int) [][]byte {
	if n <= 0 {
		n = len(s)
	}
	a := make([][]byte, n)
	var size int
	na := 0
	for len(s) > 0 {
		if na+1 >= n {
			a[na] = s
			na++
			break
		}
		_, size = utf8.DecodeRune(s)
		a[na] = s[0:size:size]
		s = s[size:]
		na++
	}
	return a[0:na]
}

// Count 对 s 中的 sep 的非重叠实例进行计数。
// 如果 sep 是一个空切片，则 Count 返回 1 + s 中 UTF-8 编码的代码点的数目。
func Count(s, sep []byte) int {
	// special case
	if len(sep) == 0 {
		return utf8.RuneCount(s) + 1
	}
	if len(sep) == 1 {
		return bytealg.Count(s, sep[0])
	}
	n := 0
	for {
		i := Index(s, sep)
		if i == -1 {
			return n
		}
		n++
		s = s[i+len(sep):]
	}
}

// Contains 判断子切片是否在 b 里。
func Contains(b, subslice []byte) bool {
	return Index(b, subslice) != -1
}

// ContainsAny 判断字符中 UTF-8 编码的代码点是否在 b 之内。
func ContainsAny(b []byte, chars string) bool {
	return IndexAny(b, chars) >= 0
}

// ContainsRune 判断 rune 是否包含在 UTF-8 编码的字节切片 b 中。.
func ContainsRune(b []byte, r rune) bool {
	return IndexRune(b, r) >= 0
}

// IndexByte 返回 b 中 c 第一个实例的索引，如果 b 中不存在 c，则返回 -1。
func IndexByte(b []byte, c byte) int {
	return bytealg.IndexByte(b, c)
}

func indexBytePortable(s []byte, c byte) int {
	for i, b := range s {
		if b == c {
			return i
		}
	}
	return -1
}

// LastIndex 返回 s 中 sep 最后一个实例的索引；如果 s 中不存在 sep，则返回 -1。
func LastIndex(s, sep []byte) int {
	n := len(sep)
	switch {
	case n == 0:
		return len(s)
	case n == 1:
		return LastIndexByte(s, sep[0])
	case n == len(s):
		if Equal(s, sep) {
			return 0
		}
		return -1
	case n > len(s):
		return -1
	}
	// 从字符串末尾搜索 Rabin-Karp
	hashss, pow := bytealg.HashStrRevBytes(sep)
	last := len(s) - n
	var h uint32
	for i := len(s) - 1; i >= last; i-- {
		h = h*bytealg.PrimeRK + uint32(s[i])
	}
	if h == hashss && Equal(s[last:], sep) {
		return last
	}
	for i := last - 1; i >= 0; i-- {
		h *= bytealg.PrimeRK
		h += uint32(s[i])
		h -= pow * uint32(s[i+n])
		if h == hashss && Equal(s[i:i+n], sep) {
			return i
		}
	}
	return -1
}

// LastIndexByte 返回 s 中 c 最后一个实例的索引；如果 s 中不存在 c，则返回 -1。
func LastIndexByte(s []byte, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// IndexRune 将 s 解释为 UTF-8 编码的代码点序列。
// 它返回给定 rune 中 s 第一次出现的字节索引。
// 如果 s 中不存在 rune，则返回 -1。
// 如果 r 为 utf8.RuneError，它将返回任一无效 UTF-8 字节序列的第一个实例。
func IndexRune(s []byte, r rune) int {
	switch {
	case 0 <= r && r < utf8.RuneSelf:
		return IndexByte(s, byte(r))
	case r == utf8.RuneError:
		for i := 0; i < len(s); {
			r1, n := utf8.DecodeRune(s[i:])
			if r1 == utf8.RuneError {
				return i
			}
			i += n
		}
		return -1
	case !utf8.ValidRune(r):
		return -1
	default:
		var b [utf8.UTFMax]byte
		n := utf8.EncodeRune(b[:], r)
		return Index(s, b[:n])
	}
}

// IndexAny 将 s 解释为 UTF-8 编码的 Unicode 代码点的序列。
// 它返回字符中任一 Unicode 代码点 s 中最后一次出现的字节索引。
// 如果字符为空或没有相同的代码点，则返回 -1。
func IndexAny(s []byte, chars string) int {
	if chars == "" {
		// 避免扫描所有 s。
		return -1
	}
	if len(s) == 1 {
		r := rune(s[0])
		if r >= utf8.RuneSelf {
			// 搜索 utf8.RuneError。
			for _, r = range chars {
				if r == utf8.RuneError {
					return 0
				}
			}
			return -1
		}
		if bytealg.IndexByteString(chars, s[0]) >= 0 {
			return 0
		}
		return -1
	}
	if len(chars) == 1 {
		r := rune(chars[0])
		if r >= utf8.RuneSelf {
			r = utf8.RuneError
		}
		return IndexRune(s, r)
	}
	if len(s) > 8 {
		if as, isASCII := makeASCIISet(chars); isASCII {
			for i, c := range s {
				if as.contains(c) {
					return i
				}
			}
			return -1
		}
	}
	var width int
	for i := 0; i < len(s); i += width {
		r := rune(s[i])
		if r < utf8.RuneSelf {
			if bytealg.IndexByteString(chars, s[i]) >= 0 {
				return i
			}
			width = 1
			continue
		}
		r, width = utf8.DecodeRune(s[i:])
		if r != utf8.RuneError {
			// r 2 到 4 个字节
			if len(chars) == width {
				if chars == string(r) {
					return i
				}
				continue
			}
			// 如果可用，使用 bytealg.IndexString 提高性能。
			if bytealg.MaxLen >= width {
				if bytealg.IndexString(chars, string(r)) >= 0 {
					return i
				}
				continue
			}
		}
		for _, ch := range chars {
			if r == ch {
				return i
			}
		}
	}
	return -1
}

// LastIndexAny 将 s 解释为 UTF-8 编码的 Unicode 代码点的序列。
// 它返回字符中任一 Unicode 代码点 s 中 最后一次出现的字节索引。
// 如果字符为空或没有相同的代码点，则返回 -1。
func LastIndexAny(s []byte, chars string) int {
	if chars == "" {
		// 避免扫描所有 s。
		return -1
	}
	if len(s) > 8 {
		if as, isASCII := makeASCIISet(chars); isASCII {
			for i := len(s) - 1; i >= 0; i-- {
				if as.contains(s[i]) {
					return i
				}
			}
			return -1
		}
	}
	if len(s) == 1 {
		r := rune(s[0])
		if r >= utf8.RuneSelf {
			for _, r = range chars {
				if r == utf8.RuneError {
					return 0
				}
			}
			return -1
		}
		if bytealg.IndexByteString(chars, s[0]) >= 0 {
			return 0
		}
		return -1
	}
	if len(chars) == 1 {
		cr := rune(chars[0])
		if cr >= utf8.RuneSelf {
			cr = utf8.RuneError
		}
		for i := len(s); i > 0; {
			r, size := utf8.DecodeLastRune(s[:i])
			i -= size
			if r == cr {
				return i
			}
		}
		return -1
	}
	for i := len(s); i > 0; {
		r := rune(s[i-1])
		if r < utf8.RuneSelf {
			if bytealg.IndexByteString(chars, s[i-1]) >= 0 {
				return i - 1
			}
			i--
			continue
		}
		r, size := utf8.DecodeLastRune(s[:i])
		i -= size
		if r != utf8.RuneError {
			// r is 2 to 4 bytes
			if len(chars) == size {
				if chars == string(r) {
					return i
				}
				continue
			}
			// 如果可用，使用 bytealg.IndexString 提高性能。
			if bytealg.MaxLen >= size {
				if bytealg.IndexString(chars, string(r)) >= 0 {
					return i
				}
				continue
			}
		}
		for _, ch := range chars {
			if r == ch {
				return i
			}
		}
	}
	return -1
}

// 通用拆分： 在 sep 的每个实例之后拆分，
// 包含子切片中 sep 的 sepSave 字节。
func genSplit(s, sep []byte, sepSave, n int) [][]byte {
	if n == 0 {
		return nil
	}
	if len(sep) == 0 {
		return explode(s, n)
	}
	if n < 0 {
		n = Count(s, sep) + 1
	}

	a := make([][]byte, n)
	n--
	i := 0
	for i < n {
		m := Index(s, sep)
		if m < 0 {
			break
		}
		a[i] = s[: m+sepSave : m+sepSave]
		s = s[m+len(sep):]
		i++
	}
	a[i] = s
	return a[:i+1]
}

// SplitN 将 s 分割成由 sep 分割的子切片，并返回这些分隔符之间的子切片。
// 如果 sep 为空，SplitN 在每个 UTF-8 序列之后分割。
// 这个数量决定了返回的子切片数:
//   n > 0: 最多 n 个子切片；最后一个子切片将是未拆分的剩余部分
//   n == 0: 结果为 nil（零子切片）
//   n < 0: 所有子切片
func SplitN(s, sep []byte, n int) [][]byte { return genSplit(s, sep, 0, n) }

// SplitAfterN 在每个 sep 实例后将 s 分隔成子切片，并返回这些子切片的切片。
// 如果 sep 为空，plitAfterN 在每个 UTF-8 序列之后分割。
// 这个数量决定了返回的子切片数:
//   n > 0: 最多 n 个子切片；最后一个子切片将是未拆分的剩余部分
//   n == 0: 结果为 nil（零子切片）
//   n < 0: 所有子切片
func SplitAfterN(s, sep []byte, n int) [][]byte {
	return genSplit(s, sep, len(sep), n)
}

// Split 将切片 s 分割为由 sep 分割的所有子切片，并返回这些分隔符之间的子切片的切片。
// 如果 sep 为空，则 Split 在每个 UTF-8 序列后拆分。
// 它等效于 SplitN，计数为 -1。
func Split(s, sep []byte) [][]byte { return genSplit(s, sep, 0, -1) }

// SplitAfter 在每个 sep 实例之后将 s 分割为所有子切片，并返回这些子切片之间的切片。
// 如果 sep 为空，则 SplitAfter 在每个 UTF-8 序列后拆分。
// 它等效于 SplitAfterN，计数为 -1。
func SplitAfter(s, sep []byte) [][]byte {
	return genSplit(s, sep, len(sep), -1)
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

// Fields 将 s 解释为 UTF-8 编码的代码点序列。
// 它按照一个或多个连续的空白字符（按照 unicode.IsSpace 的定义的字符）对切片 s 进行分割，
// 返回一个 s 的子切片，如果 s 只包含空格，则返回一个空切片。
func Fields(s []byte) [][]byte {
	// 首先计算字段。
	// 如果 s 为 ASCII，则为精确计数，否则为近似值。
	n := 0
	wasSpace := 1
	// setBits 用于跟踪在 s 字节中设置了哪些位。
	setBits := uint8(0)
	for i := 0; i < len(s); i++ {
		r := s[i]
		setBits |= r
		isSpace := int(asciiSpace[r])
		n += wasSpace & ^isSpace
		wasSpace = isSpace
	}

	if setBits >= utf8.RuneSelf {
		// 输入切片中的某些符文不是 ASCII.
		return FieldsFunc(s, unicode.IsSpace)
	}

	// ASCII 快捷路径
	a := make([][]byte, n)
	na := 0
	fieldStart := 0
	i := 0
	// 跳过输入前面的空格。
	for i < len(s) && asciiSpace[s[i]] != 0 {
		i++
	}
	fieldStart = i
	for i < len(s) {
		if asciiSpace[s[i]] == 0 {
			i++
			continue
		}
		a[na] = s[fieldStart:i:i]
		na++
		i++
		// 跳过字段之间的空格。
		for i < len(s) && asciiSpace[s[i]] != 0 {
			i++
		}
		fieldStart = i
	}
	if fieldStart < len(s) { // 最后一个字段可能以 EOF 结尾。
		a[na] = s[fieldStart:len(s):len(s)]
	}
	return a
}

// FieldsFunc 将 s 解释为 UTF-8 编码的代码点序列。
// 它在满足 f(c) 的每个代码点 c 处分割切片 s，并返回 s 的子切片。
// 如果 s 中所有代码点都满足 f(c)，或 s 的长度为 0，则返回一个空切片。
//
// FieldsFunc 不保证调用 f(c) 的顺序，并假定对于给定的 c，f 总是返回相同的值。
func FieldsFunc(s []byte, f func(rune) bool) [][]byte {
	// span 用于记录形式为 s[start:end] 的 s 的一部分。
	// 开始索引是包含的，结束索引是不包含的。
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, 32)

	// 查找字段的开始和结束索引。
	// 在单独的过程中执行此操作（而不是对字符串s进行切片
	// 并立即收集结果子字符串）
	// 可能由于缓存，效率更高。
	start := -1 // valid span start if >= 0
	for i := 0; i < len(s); {
		size := 1
		r := rune(s[i])
		if r >= utf8.RuneSelf {
			r, size = utf8.DecodeRune(s[i:])
		}
		if f(r) {
			if start >= 0 {
				spans = append(spans, span{start, i})
				start = -1
			}
		} else {
			if start < 0 {
				start = i
			}
		}
		i += size
	}

	// 最后一个字段可能以 EOF 结尾。
	if start >= 0 {
		spans = append(spans, span{start, len(s)})
	}

	// 根据记录的字段索引创建子切片。
	a := make([][]byte, len(spans))
	for i, span := range spans {
		a[i] = s[span.start:span.end:span.end]
	}

	return a
}

// Join 将 s 的元素连接起来以创建一个新的字节切片。
// 分隔符 sep 放置在所得切片中的元素之间。
func Join(s [][]byte, sep []byte) []byte {
	if len(s) == 0 {
		return []byte{}
	}
	if len(s) == 1 {
		// Just return a copy.
		return append([]byte(nil), s[0]...)
	}
	n := len(sep) * (len(s) - 1)
	for _, v := range s {
		n += len(v)
	}

	b := make([]byte, n)
	bp := copy(b, s[0])
	for _, v := range s[1:] {
		bp += copy(b[bp:], sep)
		bp += copy(b[bp:], v)
	}
	return b
}

// HasPrefix 测试字节切片 s 是否以后缀开头。
func HasPrefix(s, prefix []byte) bool {
	return len(s) >= len(prefix) && Equal(s[0:len(prefix)], prefix)
}

// HasSuffix 测试字节切片 s 是否以后缀结尾。
func HasSuffix(s, suffix []byte) bool {
	return len(s) >= len(suffix) && Equal(s[len(s)-len(suffix):], suffix)
}

// Map 返回字节切片 s 的副本，其所有字符都根据映射函数进行了修改。
// 如果映射返回负值，则将字符从字节切片中丢弃，并且不进行替换。
// s 中的字符和输出被解释为 UTF-8 编码的代码点。
func Map(mapping func(r rune) rune, s []byte) []byte {
	// 在最坏的情况下，切片会在映射时增长，让事情变得更糟。
	// 但我们很少会会假设这是好的。
	// 它也可以收缩，自然而然地消失。
	maxbytes := len(s) // b 的长度
	nbytes := 0        // 用 b 编码的字节数
	b := make([]byte, maxbytes)
	for i := 0; i < len(s); {
		wid := 1
		r := rune(s[i])
		if r >= utf8.RuneSelf {
			r, wid = utf8.DecodeRune(s[i:])
		}
		r = mapping(r)
		if r >= 0 {
			rl := utf8.RuneLen(r)
			if rl < 0 {
				rl = len(string(utf8.RuneError))
			}
			if nbytes+rl > maxbytes {
				// Grow the buffer.
				maxbytes = maxbytes*2 + utf8.UTFMax
				nb := make([]byte, maxbytes)
				copy(nb, b[0:nbytes])
				b = nb
			}
			nbytes += utf8.EncodeRune(b[nbytes:maxbytes], r)
		}
		i += wid
	}
	return b[0:nbytes]
}

// Repeat 返回一个由 b 的计数副本组成的新的字节切片。
//
// 如果 count 为负或 (len(b) * count) 的结果溢出，它会 panic。
func Repeat(b []byte, count int) []byte {
	if count == 0 {
		return []byte{}
	}
	// 由于我们无法在溢出时返回错误，
	// 如果这种重复会产生溢出，我们应该 panic。
	// 参见 Issue golang.org/issue/16237.
	if count < 0 {
		panic("bytes: negative Repeat count")
	} else if len(b)*count/count != len(b) {
		panic("bytes: Repeat count causes overflow")
	}

	nb := make([]byte, len(b)*count)
	bp := copy(nb, b)
	for bp < len(nb) {
		copy(nb[bp:], nb[:bp])
		bp *= 2
	}
	return nb
}

// ToUpper 返回字节切片 s 的副本，其中所有 Unicode 字母都映射到其大写字母。
func ToUpper(s []byte) []byte {
	isASCII, hasLower := true, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			isASCII = false
			break
		}
		hasLower = hasLower || ('a' <= c && c <= 'z')
	}

	if isASCII { // 仅针对 ASCII 字节切片进行优化。
		if !hasLower {
			// 仅返回一个副本。
			return append([]byte(""), s...)
		}
		b := make([]byte, len(s))
		for i := 0; i < len(s); i++ {
			c := s[i]
			if 'a' <= c && c <= 'z' {
				c -= 'a' - 'A'
			}
			b[i] = c
		}
		return b
	}
	return Map(unicode.ToUpper, s)
}

// 返回字节切片 s 的副本，其中所有 Unicode 字母均映射到小写字母。
func ToLower(s []byte) []byte {
	isASCII, hasUpper := true, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			isASCII = false
			break
		}
		hasUpper = hasUpper || ('A' <= c && c <= 'Z')
	}

	if isASCII { // 仅针对 ASCII 字节切片进行优化。
		if !hasUpper {
			return append([]byte(""), s...)
		}
		b := make([]byte, len(s))
		for i := 0; i < len(s); i++ {
			c := s[i]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			b[i] = c
		}
		return b
	}
	return Map(unicode.ToLower, s)
}

// ToTitle 将 s 视为 UTF-8 编码的字节，并返回一个副本，其中包含所有映射到标题大小写的 Unicode 字母。
func ToTitle(s []byte) []byte { return Map(unicode.ToTitle, s) }

// ToUpperSpecial 将 s 视为 UTF-8 编码的字节，并返回一个副本，其中所有 Unicode 字母均映射为它们的大写字母，并优先使用特殊的大小写规则。
func ToUpperSpecial(c unicode.SpecialCase, s []byte) []byte {
	return Map(c.ToUpper, s)
}

// ToLowerSpecial 将 s 视为 UTF-8 编码的字节，并返回一个副本，其中所有 Unicode 字母均映射为它们的小写字母，并优先使用特殊的大小写规则。
func ToLowerSpecial(c unicode.SpecialCase, s []byte) []byte {
	return Map(c.ToLower, s)
}

// ToTitleSpecial 将 s 视为 UTF-8 编码的字节，并返回一个副本，其中所有 Unicode 字母均映射到其标题大小写，并优先使用特殊的大小写规则。
func ToTitleSpecial(c unicode.SpecialCase, s []byte) []byte {
	return Map(c.ToTitle, s)
}

// ToValidUTF8 将 s 视为 UTF-8 编码的字节，并返回一个副本，其中每次运行的字节均表示无效的 UTF-8，并用替换的字节替换，该字节可以为空。
func ToValidUTF8(s, replacement []byte) []byte {
	b := make([]byte, 0, len(s)+len(replacement))
	invalid := false // 前一个字节来自无效的 UTF-8 序列
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			i++
			invalid = false
			b = append(b, byte(c))
			continue
		}
		_, wid := utf8.DecodeRune(s[i:])
		if wid == 1 {
			i++
			if !invalid {
				invalid = true
				b = append(b, replacement...)
			}
			continue
		}
		invalid = false
		b = append(b, s[i:i+wid]...)
		i += wid
	}
	return b
}

// isSeparator 判断该符文是否可以标记单词边界。
// TODO: 在程序包 unicode 捕获更多属性时更新。
func isSeparator(r rune) bool {
	// ASCII 字母数字和下划线不是分隔符
	if r <= 0x7F {
		switch {
		case '0' <= r && r <= '9':
			return false
		case 'a' <= r && r <= 'z':
			return false
		case 'A' <= r && r <= 'Z':
			return false
		case r == '_':
			return false
		}
		return true
	}
	// 字母和数字不是分隔符
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	// 否则，我们只能将空格视为分隔符。
	return unicode.IsSpace(r)
}

// Title 将 s 视为 UTF-8 编码的字节，并返回一个副本，其中包含所有以单词开头的 Unicode 字母，这些字母映射到它们的标题大小写。
//
// BUG(rsc):标题用于单词边界的规则不能正确处理 Unicode 标点。
func Title(s []byte) []byte {
	// 在此处使用闭包来记住状态。
	// 麻烦但有效。Depends on Map scanning in order and calling
	// the closure once per rune.依赖于 Map 扫描的顺序和每个 rune 调用闭包。
	prev := ' '
	return Map(
		func(r rune) rune {
			if isSeparator(prev) {
				prev = r
				return unicode.ToTitle(r)
			}
			prev = r
			return r
		},
		s)
}

// TrimLeftFunc 将 s 视为 UTF-8 编码的字节，并通过将前面满足 f(c) 的 UTF-8 编码的代码点 c 分割出来，返回 s 的子切片。
func TrimLeftFunc(s []byte, f func(r rune) bool) []byte {
	i := indexFunc(s, f, false)
	if i == -1 {
		return nil
	}
	return s[i:]
}

// TrimRightFunc 通过将后面满足 f(c) 的 UTF-8 编码的代码点 c 分割出来，返回 s 的子切片。
func TrimRightFunc(s []byte, f func(r rune) bool) []byte {
	i := lastIndexFunc(s, f, false)
	if i >= 0 && s[i] >= utf8.RuneSelf {
		_, wid := utf8.DecodeRune(s[i:])
		i += wid
	} else {
		i++
	}
	return s[0:i]
}

// TrimFunc 通过将所有满足 f(c) 的前、后 UTF-8 编码的代码点 c 分割出来，返回 s 的子切片。
func TrimFunc(s []byte, f func(r rune) bool) []byte {
	return TrimRightFunc(TrimLeftFunc(s, f), f)
}

// TrimPrefix 返回不包含给定前缀字符串的 s。
// 如果 s 不以前缀开头，则 s 不变。
func TrimPrefix(s, prefix []byte) []byte {
	if HasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

// TrimSuffix 返回不包含给定后缀字符串的 s。
// 如果 s 不以后缀结尾，则 s 不变。
func TrimSuffix(s, suffix []byte) []byte {
	if HasSuffix(s, suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

// IndexFunc 将 s 解释为 UTF-8 编码的代码点序列。
// 它返回 s 中满足 f(c) 的第一个 Unicode 代码点的的字节索引，如果没有，则返回 -1。
func IndexFunc(s []byte, f func(r rune) bool) int {
	return indexFunc(s, f, true)
}

// LastIndexFunc 将 s 解释为一系列 UTF-8 编码的代码点。
// 它返回 s 中满足 f(c) 的最后一个 Unicode 代码点的的字节索引，如果没有，则返回 -1。
func LastIndexFunc(s []byte, f func(r rune) bool) int {
	return lastIndexFunc(s, f, true)
}

// indexFunc 和 IndexFunc 是一样的，除了 truth == false，
// 谓语函数的意义颠倒了。
func indexFunc(s []byte, f func(r rune) bool, truth bool) int {
	start := 0
	for start < len(s) {
		wid := 1
		r := rune(s[start])
		if r >= utf8.RuneSelf {
			r, wid = utf8.DecodeRune(s[start:])
		}
		if f(r) == truth {
			return start
		}
		start += wid
	}
	return -1
}

// lastIndexFunc 和 LastIndexFunc 是一样的，除了 truth == false，
// 谓语函数的意义颠倒了。
func lastIndexFunc(s []byte, f func(r rune) bool, truth bool) int {
	for i := len(s); i > 0; {
		r, size := rune(s[i-1]), 1
		if r >= utf8.RuneSelf {
			r, size = utf8.DecodeLastRune(s[0:i])
		}
		i -= size
		if f(r) == truth {
			return i
		}
	}
	return -1
}

// asciiSet 是一个 32 字节的值，其中每个位代表一个给定集合中的 ASCII 字符。
// 低位 16 字节的 128 位，从最低单词的最低有效位开始到最高单词的最高有效位，
// 映射到所有 128 个 ASCII 字符的全范围。高位 16 字节的 128 位将被归零，
// 以确保任何非 ASCII 字符将被判断为不在集合中。
type asciiSet [8]uint32

// makeASCIISet 创建一个 ASCII 字符的集合，并判断所有的字符中的字符是否是 ASCII。
func makeASCIISet(chars string) (as asciiSet, ok bool) {
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		if c >= utf8.RuneSelf {
			return as, false
		}
		as[c>>5] |= 1 << uint(c&31)
	}
	return as, true
}

// contains 判断 c 是否在集合里。
func (as *asciiSet) contains(c byte) bool {
	return (as[c>>5] & (1 << uint(c&31))) != 0
}

func makeCutsetFunc(cutset string) func(r rune) bool {
	if len(cutset) == 1 && cutset[0] < utf8.RuneSelf {
		return func(r rune) bool {
			return r == rune(cutset[0])
		}
	}
	if as, isASCII := makeASCIISet(cutset); isASCII {
		return func(r rune) bool {
			return r < utf8.RuneSelf && as.contains(byte(r))
		}
	}
	return func(r rune) bool {
		for _, c := range cutset {
			if c == r {
				return true
			}
		}
		return false
	}
}

// Trim 通过分割所有前导和末尾的 UTF-8 编码的代码点来返回 s 的子切片。
func Trim(s []byte, cutset string) []byte {
	return TrimFunc(s, makeCutsetFunc(cutset))
}

// TrimLeft 通过分割所有前导的 UTF-8 编码的代码点来返回 s 的子切片。
func TrimLeft(s []byte, cutset string) []byte {
	return TrimLeftFunc(s, makeCutsetFunc(cutset))
}

// TrimRight 通过分割所有末尾的 UTF-8 编码的代码点来返回 s 的子切片。
func TrimRight(s []byte, cutset string) []byte {
	return TrimRightFunc(s, makeCutsetFunc(cutset))
}

// 根据 Unicode 的定义，TrimSpace 通过将所有前导和末尾的空格分割，返回 s 的一个子切片。
func TrimSpace(s []byte) []byte {
	// ASCII 的快速路径：查找第一个 ASCII 非空格字节
	start := 0
	for ; start < len(s); start++ {
		c := s[start]
		if c >= utf8.RuneSelf {
			// 如果遇到一个非 ASCII 字节, 在剩余的字节上回退到
			// 较慢的 unicode 感知方法
			return TrimFunc(s[start:], unicode.IsSpace)
		}
		if asciiSpace[c] == 0 {
			break
		}
	}

	// 现在从末尾查找第一个 ASCII 非空格字节
	stop := len(s)
	for ; stop > start; stop-- {
		c := s[stop-1]
		if c >= utf8.RuneSelf {
			return TrimFunc(s[start:stop], unicode.IsSpace)
		}
		if asciiSpace[c] == 0 {
			break
		}
	}

	// 此时 s[start:stop] 以 ASCII 非空格字节开始和结束，
	// 到此完成。非 ASCII 情况在上面已经处理过了。
	if start == stop {
		// 保留以前的 TrimLeftFunc 行为的特殊情况，
		// 如果都是空格则返回 nil 而不是空切片。
		return nil
	}
	return s[start:stop]
}

// Runes 将 s 解释为 UTF-8 编码的代码点序列。
// 它返回等于 s 的一部分 rune（Unicode 代码点）的切片。
func Runes(s []byte) []rune {
	t := make([]rune, utf8.RuneCount(s))
	i := 0
	for len(s) > 0 {
		r, l := utf8.DecodeRune(s)
		t[i] = r
		i++
		s = s[l:]
	}
	return t
}

// Replace 返回切片 s 的一个副本，其中前 n 个旧的非重叠实例被新的替换。
// 如果旧的为空，则在切片的开头和每个 UTF-8 序列之后匹配，一个 k-rune 切片最多产生 k+1 个替换。
// 如果 n < 0，则替换次数没有限制。
func Replace(s, old, new []byte, n int) []byte {
	m := 0
	if n != 0 {
		// 计算替换次数。
		m = Count(s, old)
	}
	if m == 0 {
		// 只返回一个副本。
		return append([]byte(nil), s...)
	}
	if n < 0 || m < n {
		n = m
	}

	// 将替换应用于缓冲区。
	t := make([]byte, len(s)+n*(len(new)-len(old)))
	w := 0
	start := 0
	for i := 0; i < n; i++ {
		j := start
		if len(old) == 0 {
			if i > 0 {
				_, wid := utf8.DecodeRune(s[start:])
				j += wid
			}
		} else {
			j += Index(s[start:], old)
		}
		w += copy(t[w:], s[start:j])
		w += copy(t[w:], new)
		start = j + len(old)
	}
	w += copy(t[w:], s[start:])
	return t[0:w]
}

// ReplaceAll 返回切片 s 的一个副本，其中所有旧的非重叠实例都被新的替换。
// 如果旧的为空， 则在切片的开头和每个 UTF-8 序列之后匹配，一个 k-rune 切片最多产生 k+1 个替换。
func ReplaceAll(s, old, new []byte) []byte {
	return Replace(s, old, new, -1)
}

// EqualFold 判断 s 和 t 是否被解释为 UTF-8 字符串,
// 在 Unicode 大小写折叠下是相等的，不区分大小写的情况下，这是通用的。
func EqualFold(s, t []byte) bool {
	for len(s) != 0 && len(t) != 0 {
		// 从每个字符串中提取第一个 rune。
		var sr, tr rune
		if s[0] < utf8.RuneSelf {
			sr, s = rune(s[0]), s[1:]
		} else {
			r, size := utf8.DecodeRune(s)
			sr, s = r, s[size:]
		}
		if t[0] < utf8.RuneSelf {
			tr, t = rune(t[0]), t[1:]
		} else {
			r, size := utf8.DecodeRune(t)
			tr, t = r, t[size:]
		}

		// 如果匹配就继续；反之，则返回false。

		// 简单的情况。
		if tr == sr {
			continue
		}

		// 使 sr < tr 简化为如下内容。
		if tr < sr {
			tr, sr = sr, tr
		}
		// 快速检查 ASCII。
		if tr < utf8.RuneSelf {
			// 仅 ASCII，sr/tr 必须为大/小写
			if 'A' <= sr && sr <= 'Z' && tr == sr+'a'-'A' {
				continue
			}
			return false
		}

		// 通常情况。SimpleFold(x) 返回大于 x 的 下一个等效 rune，
		// 或换成较小的值。
		r := unicode.SimpleFold(sr)
		for r != sr && r < tr {
			r = unicode.SimpleFold(r)
		}
		if r == tr {
			continue
		}
		return false
	}

	// One string is empty. Are both?
	return len(s) == len(t)
}

// Index 返回 s 中 sep 第一个实例的索引，如果 s 中不存在 sep，则返回 -1。
func Index(s, sep []byte) int {
	n := len(sep)
	switch {
	case n == 0:
		return 0
	case n == 1:
		return IndexByte(s, sep[0])
	case n == len(s):
		if Equal(sep, s) {
			return 0
		}
		return -1
	case n > len(s):
		return -1
	case n <= bytealg.MaxLen:
		// 当 s 和 sep 都较小时使用蛮力
		if len(s) <= bytealg.MaxBruteForce {
			return bytealg.Index(s, sep)
		}
		c0 := sep[0]
		c1 := sep[1]
		i := 0
		t := len(s) - n + 1
		fails := 0
		for i < t {
			if s[i] != c0 {
				// IndexByte 比 bytealg.Index 快,
				// 因此只要我们没有得到很多报错，就可以使用它。
				o := IndexByte(s[i+1:t], c0)
				if o < 0 {
					return -1
				}
				i += o + 1
			}
			if s[i+1] == c1 && Equal(s[i:i+n], sep) {
				return i
			}
			fails++
			i++
			// 当 IndexByte 产生太多的报错时，使用 bytealg.Index。
			if fails > bytealg.Cutover(i) {
				r := bytealg.Index(s[i:], sep)
				if r >= 0 {
					return r + i
				}
				return -1
			}
		}
		return -1
	}
	c0 := sep[0]
	c1 := sep[1]
	i := 0
	fails := 0
	t := len(s) - n + 1
	for i < t {
		if s[i] != c0 {
			o := IndexByte(s[i+1:t], c0)
			if o < 0 {
				break
			}
			i += o + 1
		}
		if s[i+1] == c1 && Equal(s[i:i+n], sep) {
			return i
		}
		i++
		fails++
		if fails >= 4+i>>4 && i < t {
			// 放弃 IndexByte, 它跳得不够远
			// 比不上 Rabin-Karp.
			// 实验（使用 IndexPeriodic）建议切换大约跳过 16 个字节。
			// TODO: 如果 sep 的大前缀匹配，
			// 我们应该以更大的平均跳过来切换，
			// 因为 Equal 变得更加昂贵。
			// 此代码未考虑到这种影响。
			j := bytealg.IndexRabinKarpBytes(s[i:], sep)
			if j < 0 {
				return -1
			}
			return i + j
		}
	}
	return -1
}
