// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unicode

// IsDigit 报告码点 r 是否为十进制数字。
func IsDigit(r rune) bool {
	if r <= MaxLatin1 {
		return '0' <= r && r <= '9'
	}
	return isExcludingLatin(Digit, r)
}
