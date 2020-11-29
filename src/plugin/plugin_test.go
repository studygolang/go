// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !linux linux,!arm64

package plugin_test

import (
	_ "plugin"
	"testing"
)

func TestPlugin(t *testing.T) {
	// 本测试确保导入 plugin 包的可执行文件确实能执行。具体参见 issue #28789。
}
