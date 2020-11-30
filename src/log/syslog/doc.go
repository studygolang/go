// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// syslog 包为系统日志服务提供了一个简单接口。
// 它可以使用 UNIX 域套接字，UDP 或 TCP 将消息发送到 syslog 守护程序。
//
// 仅调用一次 Dial 是必要的。
// 在写失败场景，syslog 客户端会尝试重新连接服务器并再次写入。
//
// syslog 包已冻结，并且不接受新功能。
// 一些外部软件包提供了更多功能。可以查看：
//
//   https://godoc.org/?q=syslog
package syslog

// BUG(brainman): Windows 上未实现此包。
// 由于 syslog 包被冻结，Windows 用户被鼓励去使用标准库之外的包。
// 供参考，查看 https://golang.org/issue/1108。

// BUG(akumar): Plan 9 上未实现此包。
