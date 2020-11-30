// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO: 本文档包含土耳其语和阿塞拜疆语的特殊大小写规则。
// 该文档应包含有特殊大小写规则的所有语言且应自动生成，但这又得先开发些API。

package unicode

var TurkishCase SpecialCase = _TurkishCase
var _TurkishCase = SpecialCase{
	CaseRange{0x0049, 0x0049, d{0, 0x131 - 0x49, 0}},
	CaseRange{0x0069, 0x0069, d{0x130 - 0x69, 0, 0x130 - 0x69}},
	CaseRange{0x0130, 0x0130, d{0, 0x69 - 0x130, 0}},
	CaseRange{0x0131, 0x0131, d{0x49 - 0x131, 0, 0x49 - 0x131}},
}

var AzeriCase SpecialCase = _TurkishCase
