// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strconv

// ParseBool 返回字符串表示的bool值
// 可接受字符串 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False.
// 其他的字符串会报错
func ParseBool(str string) (bool, error) {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False":
		return false, nil
	}
	return false, syntaxError("ParseBool", str)
}

// FormatBool 根据b的值返回 "true" 或 "false" 的字符串
func FormatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// AppendBool 根据b的值选择字符串 "true" 或 "false" 添加到 dst 中，并返回扩展后的 slice。
// 等价于 append(dst,FormatBool(b)...)
func AppendBool(dst []byte, b bool) []byte {
	if b {
		return append(dst, "true"...)
	}
	return append(dst, "false"...)
}
