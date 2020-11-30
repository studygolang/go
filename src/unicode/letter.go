// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// unicode 包提供数据和函数以测试 Unicode 码点的某些属性。
package unicode

const (
	MaxRune         = '\U0010FFFF' // Unicode 码点最大值。
	ReplacementChar = '\uFFFD'     // 代表非法码点。
	MaxASCII        = '\u007F'     // ASCII 最大值。
	MaxLatin1       = '\u00FF'     // Latin-1 最大值。
)

// RangeTable 通过列出 Unicode 码点集的范围来定义码点。为了节省空间，它在两个切片中列出了码点范围：表示16位和32位码点的切片。这两个片必须按顺序排列且不能重叠。R32 应该只包含 >= 0x10000（1 << 16）的值。
type RangeTable struct {
	R16         []Range16
	R32         []Range32
	LatinOffset int // R16 中 Hi <= MaxLatin1 的条目数。
}

// Range16 表示一系列16位 Unicode 码点，范围从 Lo 到 Hi且具有指定的跨度。
type Range16 struct {
	Lo     uint16
	Hi     uint16
	Stride uint16
}

// Range32 表示一系列 Unicode 码点。当一个或多个值不适合16位表示时使用 Range32，它的范围从 Lo 到 Hi 且具有指定的跨度。 Lo 和 Hi 必须始终为 >= 1 << 16。
type Range32 struct {
	Lo     uint32
	Hi     uint32
	Stride uint32
}

// CaseRange 表示用于简单大小写转换的 Unicode 码点范围，范围从 Lo 到 Hi，跨度为1。Delta 是大小写转换过程中大小写码点值的差，需要加到该码点上以转换，其值可为负数。
// 有一种特殊情况表示交替的大小写对，它以 {UpperLower，UpperLower，UpperLower} 加一固定Delta表示。
// 常量 UpperLower 带有不可能的 Delta 值。
type CaseRange struct {
	Lo    uint32
	Hi    uint32
	Delta d
}

// SpecialCase 表示特定语言的大小写映射，例如土耳其语。
// SpecialCase 的函数自定义了标准大小写映射。
type SpecialCase []CaseRange

// BUG(r): 对于输入或输出中有多个码点的字符而言没有用于全大小写折叠的机制。

// 加到 CaseRanges 内 Delta 上以进行大小写映射的指标。
const (
	UpperCase = iota
	LowerCase
	TitleCase
	MaxCase
)

type d [MaxCase]rune // 使 CaseRanges 文本更短。

// 若 CaseRange 的Delta字段为 UpperLower，则表示此 CaseRange 是 Upper Lower Upper Lower Lower序列。
const (
	UpperLower = MaxRune + 1 // （不可能是合法的 delta.）
)

// linearMax 是用于线性搜索非 Latin1 码点的表的最大值。
// 运行 'go test -calibrate' 得到。
const linearMax = 18

// is16 报告 r 是否在16位排序切片范围内。
func is16(ranges []Range16, r uint16) bool {
	if len(ranges) <= linearMax || r <= MaxLatin1 {
		for i := range ranges {
			range_ := &ranges[i]
			if r < range_.Lo {
				return false
			}
			if r <= range_.Hi {
				return range_.Stride == 1 || (r-range_.Lo)%range_.Stride == 0
			}
		}
		return false
	}

	// 在 ranges 上进行 二分查找。
	lo := 0
	hi := len(ranges)
	for lo < hi {
		m := lo + (hi-lo)/2
		range_ := &ranges[m]
		if range_.Lo <= r && r <= range_.Hi {
			return range_.Stride == 1 || (r-range_.Lo)%range_.Stride == 0
		}
		if r < range_.Lo {
			hi = m
		} else {
			lo = m + 1
		}
	}
	return false
}

// is32 报告 r 是否在32位排序切片范围内。
func is32(ranges []Range32, r uint32) bool {
	if len(ranges) <= linearMax {
		for i := range ranges {
			range_ := &ranges[i]
			if r < range_.Lo {
				return false
			}
			if r <= range_.Hi {
				return range_.Stride == 1 || (r-range_.Lo)%range_.Stride == 0
			}
		}
		return false
	}

	// 在 ranges 上进行 二分查找。
	lo := 0
	hi := len(ranges)
	for lo < hi {
		m := lo + (hi-lo)/2
		range_ := ranges[m]
		if range_.Lo <= r && r <= range_.Hi {
			return range_.Stride == 1 || (r-range_.Lo)%range_.Stride == 0
		}
		if r < range_.Lo {
			hi = m
		} else {
			lo = m + 1
		}
	}
	return false
}

// Is 报告码点是否在指定的范围表中。
func Is(rangeTab *RangeTable, r rune) bool {
	r16 := rangeTab.R16
	if len(r16) > 0 && r <= rune(r16[len(r16)-1].Hi) {
		return is16(r16, uint16(r))
	}
	r32 := rangeTab.R32
	if len(r32) > 0 && r >= rune(r32[0].Lo) {
		return is32(r32, uint32(r))
	}
	return false
}

func isExcludingLatin(rangeTab *RangeTable, r rune) bool {
	r16 := rangeTab.R16
	if off := rangeTab.LatinOffset; len(r16) > off && r <= rune(r16[len(r16)-1].Hi) {
		return is16(r16[off:], uint16(r))
	}
	r32 := rangeTab.R32
	if len(r32) > 0 && r >= rune(r32[0].Lo) {
		return is32(r32, uint32(r))
	}
	return false
}

// IsUpper 报告该码点是否为大写字母。
func IsUpper(r rune) bool {
	// 请参见 IsGraphic 处的评论。
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pLmask == pLu
	}
	return isExcludingLatin(Upper, r)
}

// IsLower 报告该码点是否为小写字母。
func IsLower(r rune) bool {
	// 请参见 IsGraphic 处的评论。
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pLmask == pLl
	}
	return isExcludingLatin(Lower, r)
}

// IsTitle 报告该码点是否为大小写字母。
func IsTitle(r rune) bool {
	if r <= MaxLatin1 {
		return false
	}
	return isExcludingLatin(Title, r)
}

// to 函数映射使用了特殊大小写映射的码点。
// 该函数还报告 caseRange 是否包含 r 的映射。
func to(_case int, r rune, caseRange []CaseRange) (mappedRune rune, foundMapping bool) {
	if _case < 0 || MaxCase <= _case {
		return ReplacementChar, false // 任何合理的错误。
	}
	// 在 ranges 上进行二分搜索。
	lo := 0
	hi := len(caseRange)
	for lo < hi {
		m := lo + (hi-lo)/2
		cr := caseRange[m]
		if rune(cr.Lo) <= r && r <= rune(cr.Hi) {
			delta := cr.Delta[_case]
			if delta > MaxRune {
				// 在一个 Upper-Lower 序列，通常以大写开沟，实际的 deltas 总是像这样：
				//	{0, 1, 0}    大写（小写随后）
				//	{-1, 0, -1}  小写（大写，首字母大写在前）
				// 从序列开始，偶数位为大写，奇数位为小写。
				// 清除或设置序列偏移中的位参数可得到正确的映射。
				// 常量 UpperCase 和 TitleCase 为偶，LowerCase 为奇，所以从 _case 取低位。
				return rune(cr.Lo) + ((r-rune(cr.Lo))&^1 | rune(_case&1)), true
			}
			return r + delta, true
		}
		if r < rune(cr.Lo) {
			hi = m
		} else {
			lo = m + 1
		}
	}
	return r, false
}

// To 函数将码点和特定大小写相互映射：UpperCase， LowerCase， TitleCase。
func To(_case int, r rune) rune {
	r, _ = to(_case, r, CaseRanges)
	return r
}

// ToUpper 将码点映射到大写。
func ToUpper(r rune) rune {
	if r <= MaxASCII {
		if 'a' <= r && r <= 'z' {
			r -= 'a' - 'A'
		}
		return r
	}
	return To(UpperCase, r)
}

// ToLower 将码点映射到小写。
func ToLower(r rune) rune {
	if r <= MaxASCII {
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
		return r
	}
	return To(LowerCase, r)
}

// ToTitle 将码点映射到首字母大写。
func ToTitle(r rune) rune {
	if r <= MaxASCII {
		if 'a' <= r && r <= 'z' { // title case is upper case for ASCII
			r -= 'a' - 'A'
		}
		return r
	}
	return To(TitleCase, r)
}

// ToUpper 将码点映射到大写，并优先使用特殊映射。
func (special SpecialCase) ToUpper(r rune) rune {
	r1, hadMapping := to(UpperCase, r, []CaseRange(special))
	if r1 == r && !hadMapping {
		r1 = ToUpper(r)
	}
	return r1
}

// ToTitle 将码点映射到首字母大写，并优先使用特殊映射。
func (special SpecialCase) ToTitle(r rune) rune {
	r1, hadMapping := to(TitleCase, r, []CaseRange(special))
	if r1 == r && !hadMapping {
		r1 = ToTitle(r)
	}
	return r1
}

// ToLower 将码点映射到小写，并优先使用特殊映射。
func (special SpecialCase) ToLower(r rune) rune {
	r1, hadMapping := to(LowerCase, r, []CaseRange(special))
	if r1 == r && !hadMapping {
		r1 = ToLower(r)
	}
	return r1
}

// caseOrbit 在 table.go 中定义为 []foldPair。现在，所有条目都符合 uint16，因此请使用 uint16。如果情况有变，则编译会失败（复合量中的常量不适用于 uint16），并且此处的类型也可更改为 uint32。
type foldPair struct {
	From uint16
	To   uint16
}

// SimpleFold 依据简单大小写折叠规则在 Unicode 码点上进行迭代。
// 对等效于 rune 的码点（包括 rune 自身），SimpleFold 返回大于 r 且存在的最小 rune，否则返回 >= 0 的最小 rune。若 r 非法，则返回 r。
//
// 例如：
//	SimpleFold('A') = 'a'
//	SimpleFold('a') = 'A'
//
//	SimpleFold('K') = 'k'
//	SimpleFold('k') = '\u212A' (Kelvin symbol, K)
//	SimpleFold('\u212A') = 'K'
//
//	SimpleFold('1') = '1'
//
//	SimpleFold(-2) = -2
//
func SimpleFold(r rune) rune {
	if r < 0 || r > MaxRune {
		return r
	}

	if int(r) < len(asciiFold) {
		return rune(asciiFold[r])
	}

	// 特殊大小写请参考 caseOrbit 表。
	lo := 0
	hi := len(caseOrbit)
	for lo < hi {
		m := lo + (hi-lo)/2
		if rune(caseOrbit[m].From) < r {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo < len(caseOrbit) && rune(caseOrbit[lo].From) == r {
		return rune(caseOrbit[lo].To)
	}

	// 没有指定折叠方法。
    // 此为具有一个或两个元素的类，如果与码点不同，则该类包含 rune、ToLower(rune) 和 ToUpper(rune)。
	if l := ToLower(r); l != r {
		return l
	}
	return ToUpper(r)
}
