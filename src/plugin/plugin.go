// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// plugin 包实现 Go 插件加载和符号解析。
//
// 一个插件就是一个 Go 主程序包，它具有导出函数和变量，这些函数和变量是使用以下命令构建的：
//
//	go build -buildmode=plugin
//
// 首次打开一个插件时（此时主函数尚未运行），将调用所有包中的 init 函数，这些 init 函数不包含在程序中。
// 一个插件只初始化一次且可以被关闭。
package plugin

// Plugin 是一个已加载的 Go 插件。
type Plugin struct {
	pluginpath string
	err        string        // 插件加载失败时设置错误值
	loaded     chan struct{} // 加载后关闭
	syms       map[string]interface{}
}

// Open 打开一个 Go 插件，如果插件所在路径已打开，则返回现存的 *Plugin。
// Open 是并发安全的。
func Open(path string) (*Plugin, error) {
	return open(path)
}

// Lookup 在插件 p 中搜索名为 symName 的符号，符号是任何导出变量或函数。
// 如果找不到该符号，Lookup 将报告错误。
// Lookup 是并发安全的。
func (p *Plugin) Lookup(symName string) (Symbol, error) {
	return lookup(p, symName)
}

// Symbol 是一个指向变量或函数的指针。
//
// 例如，一个以如下代码定义的插件
//
//	package main
//
//	import "fmt"
//
//	var V int
//
//	func F() { fmt.Printf("Hello, number %d\n", V) }
//
// 可使用 Open 函数导入，然后就能访问导出符号 V 和 F。
//
//	p, err := plugin.Open("plugin_name.so")
//	if err != nil {
//		panic(err)
//	}
//	v, err := p.Lookup("V")
//	if err != nil {
//		panic(err)
//	}
//	f, err := p.Lookup("F")
//	if err != nil {
//		panic(err)
//	}
//	*v.(*int) = 7
//	f.(func())() // 打印 "Hello, number 7"
type Symbol interface{}
