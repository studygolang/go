// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strconv

const fnParseComplex = "ParseComplex"

// convErr 将 parseFloatPrefix 返回的错误转换为 ParseComplex 需要返回的错误（s 不匹配规则或实部、虚部超出范围）
func convErr(err error, s string) (syntax, range_ error) {
	if x, ok := err.(*NumError); ok {
		x.Func = fnParseComplex
		x.Num = s
		if x.Err == ErrRange {
			return nil, x
		}
	}
	return err, nil
}

// ParseComplex 返回字符串 s 表示的复数。
// 使用 bitSize 设置复数精度：64 表示 complex64，128 表示 complex128。
// 当 bitSize = 64 时，返回值类型依然为 complex128，但是可以在不改变值的情况下转换为 complex64
//
// s 表示的数必须是 N、Ni 或 N ± Ni 的形式，其中 N 为 ParseFloat 可以识别的浮点数，i 为虚部。
// 如果第二个 N 是无符号的，则两个分量之间需要一个加号，用 ± 号表示。
// 如果第二个 N 是 NaN，则只接受一个加号。
// s 中可以包含括号但是不能包含空格.
// 得到的复数是实部和虚部由 ParseFloat 转换得到的。
//
// ParseComplex 返回的错误类型是 *NumError，其中 err.Num = s。
// 如果 s 表示的字符串不符合规则，那么 ParseComplex 返回的错误中 err.Err = ErrSyntax。
//
// 如果 s 表示的字符串符合规则，但是当 s 中实部或虚部的值大于指定浮点数限定值 1/2 ULP 时，
// ParseComplex 返回的错误中 err.Err = ErrRange 和 c = ±Inf。
func ParseComplex(s string, bitSize int) (complex128, error) {
	size := 128
	if bitSize == 64 {
		size = 32 // complex64 使用 float32 表示复数的实部和虚部
	}

	orig := s

	// 删除括号
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		s = s[1 : len(s)-1]
	}

	var pending error // pending range error, or nil

	// 读取实部（如果后面跟着 i，可能是虚部）
	re, n, err := parseFloatPrefix(s, size)
	if err != nil {
		err, pending = convErr(err, orig)
		if err != nil {
			return 0, err
		}
	}
	s = s[n:]

	// 如果没有了，结束
	if len(s) == 0 {
		return complex(re, 0), pending
	}

	// 否则，处理接下来的字符
	switch s[0] {
	case '+':
		// 使用 '+' 避免 "+NaNi" 导致错误，但只有在没有 "++" 时才有效果
		if len(s) > 1 && s[1] != '+' {
			s = s[1:]
		}
	case '-':
		// ok
	case 'i':
		// 如果 'i' 是最后一个字符，那么该复数只有虚部
		if len(s) == 1 {
			return complex(0, re), pending
		}
		fallthrough
	default:
		return 0, syntaxError(fnParseComplex, orig)
	}

	// 读取虚部
	im, n, err := parseFloatPrefix(s, size)
	if err != nil {
		err, pending = convErr(err, orig)
		if err != nil {
			return 0, err
		}
	}
	s = s[n:]
	if s != "i" {
		return 0, syntaxError(fnParseComplex, orig)
	}
	return complex(re, im), pending
}
