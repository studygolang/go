// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !windows,!plan9

package syslog

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// 优先级是 syslog 设备和严重程度的组合。
// 例如，LOG_ALERT | LOG_FTP 从 FTP 设备发送 alert 级别的消息。
// 默认的严重度是 LOG_EMERG。默认的设备是 LOG_KERN。
type Priority int

const severityMask = 0x07
const facilityMask = 0xf8

const (
	// 严重程度

	// 来自 /usr/include/sys/syslog.h
	// 在 Linux, BSD 和 OS X 上是相同的
	LOG_EMERG Priority = iota
	LOG_ALERT
	LOG_CRIT
	LOG_ERR
	LOG_WARNING
	LOG_NOTICE
	LOG_INFO
	LOG_DEBUG
)

const (
	// 设备

	// 来自 /usr/include/sys/syslog.h.
	// 在 Linux, BSD 和 OS X 上 LOG_FTP 是相同的
	LOG_KERN Priority = iota << 3
	LOG_USER
	LOG_MAIL
	LOG_DAEMON
	LOG_AUTH
	LOG_SYSLOG
	LOG_LPR
	LOG_NEWS
	LOG_UUCP
	LOG_CRON
	LOG_AUTHPRIV
	LOG_FTP
	_ // 未使用
	_ // 未使用
	_ // 未使用
	_ // 未使用
	LOG_LOCAL0
	LOG_LOCAL1
	LOG_LOCAL2
	LOG_LOCAL3
	LOG_LOCAL4
	LOG_LOCAL5
	LOG_LOCAL6
	LOG_LOCAL7
)

// Writer 是与 syslog 服务器的连接。
type Writer struct {
	priority Priority
	tag      string
	hostname string
	network  string
	raddr    string

	mu   sync.Mutex // guards conn
	conn serverConn
}

// This interface and the separate syslog_unix.go file exist for
// Solaris support as implemented by gccgo. On Solaris you cannot
// simply open a TCP connection to the syslog daemon. The gccgo
// sources have a syslog_solaris.go file that implements unixSyslog to
// return a type that satisfies this interface and simply calls the C
// library syslog function.
type serverConn interface {
	writeString(p Priority, hostname, tag, s, nl string) error
	close() error
}

type netConn struct {
	local bool
	conn  net.Conn
}

// New 建立一个与系统日志守护进程的新连接。
// 向返回的 writer 每次写操作都会发送一条日志信息，这条日志附带给定的优先级（syslog 设备和严重程度的组合）和前缀标签。
// 如果标签是空的，os.Args[0] 被用作标签。
func New(priority Priority, tag string) (*Writer, error) {
	return Dial("", "", priority, tag)
}

// Dial 通过连接到指定 network 上的 raddr 地址建立与日志守护进程的连接。
// 向返回的 writer 每次写操作都会发送一条日志信息，这条日志附带设备和严重程度（来自优先级）和标签
// 如果标签是空的，os.Args[0] 被用作标签。
// 如果 network 是空，Dial 会与本地日志服务连接。
// 其他有关 network 和 raddr 有效值，请查看 net.Dial 的文档。
func Dial(network, raddr string, priority Priority, tag string) (*Writer, error) {
	if priority < 0 || priority > LOG_LOCAL7|LOG_DEBUG {
		return nil, errors.New("log/syslog: invalid priority")
	}

	if tag == "" {
		tag = os.Args[0]
	}
	hostname, _ := os.Hostname()

	w := &Writer{
		priority: priority,
		tag:      tag,
		hostname: hostname,
		network:  network,
		raddr:    raddr,
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	err := w.connect()
	if err != nil {
		return nil, err
	}
	return w, err
}

// connect makes a connection to the syslog server.
// It must be called with w.mu held.
func (w *Writer) connect() (err error) {
	if w.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		w.conn.close()
		w.conn = nil
	}

	if w.network == "" {
		w.conn, err = unixSyslog()
		if w.hostname == "" {
			w.hostname = "localhost"
		}
	} else {
		var c net.Conn
		c, err = net.Dial(w.network, w.raddr)
		if err == nil {
			w.conn = &netConn{conn: c}
			if w.hostname == "" {
				w.hostname = c.LocalAddr().String()
			}
		}
	}
	return
}

// Write 发送日志信息到 syslog 守护进程。
func (w *Writer) Write(b []byte) (int, error) {
	return w.writeAndRetry(w.priority, string(b))
}

// Close 关闭与 syslog 守护进程的连接。
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		err := w.conn.close()
		w.conn = nil
		return err
	}
	return nil
}

// Emerg 发送一条 LOG_EMERG 严重程度的信息，通过 New 创建的连接发送时忽略严重程度。
func (w *Writer) Emerg(m string) error {
	_, err := w.writeAndRetry(LOG_EMERG, m)
	return err
}

// Alert 发送一条 LOG_ALERT 严重程度的信息，通过 New 创建的连接发送时忽略严重程度。
func (w *Writer) Alert(m string) error {
	_, err := w.writeAndRetry(LOG_ALERT, m)
	return err
}

// Crit 发送一条 LOG_CRIT 严重程度的信息，通过 New 创建的连接发送时忽略严重程度。
func (w *Writer) Crit(m string) error {
	_, err := w.writeAndRetry(LOG_CRIT, m)
	return err
}

// Err 发送一条 LOG_ERR 严重程度的信息，通过 New 创建的连接发送时忽略严重程度。
func (w *Writer) Err(m string) error {
	_, err := w.writeAndRetry(LOG_ERR, m)
	return err
}

// Warning 发送一条 LOG_WARNING 严重程度的信息，通过 New 创建的连接发送时忽略严重程度。
func (w *Writer) Warning(m string) error {
	_, err := w.writeAndRetry(LOG_WARNING, m)
	return err
}

// Notice 发送一条 LOG_NOTICE 严重程度的信息，通过 New 创建的连接发送时忽略严重程度。
func (w *Writer) Notice(m string) error {
	_, err := w.writeAndRetry(LOG_NOTICE, m)
	return err
}

// Info 发送一条 LOG_INFO 严重程度的信息，通过 New 创建的连接发送时忽略严重程度。
func (w *Writer) Info(m string) error {
	_, err := w.writeAndRetry(LOG_INFO, m)
	return err
}

// Debug 发送一条 LOG_DEBUG 严重程度的信息，通过 New 创建的连接发送时忽略严重程度。
func (w *Writer) Debug(m string) error {
	_, err := w.writeAndRetry(LOG_DEBUG, m)
	return err
}

func (w *Writer) writeAndRetry(p Priority, s string) (int, error) {
	pr := (w.priority & facilityMask) | (p & severityMask)

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		if n, err := w.write(pr, s); err == nil {
			return n, err
		}
	}
	if err := w.connect(); err != nil {
		return 0, err
	}
	return w.write(pr, s)
}

// write generates and writes a syslog formatted string. The
// format is as follows: <PRI>TIMESTAMP HOSTNAME TAG[PID]: MSG
func (w *Writer) write(p Priority, msg string) (int, error) {
	// ensure it ends in a \n
	nl := ""
	if !strings.HasSuffix(msg, "\n") {
		nl = "\n"
	}

	err := w.conn.writeString(p, w.hostname, w.tag, msg, nl)
	if err != nil {
		return 0, err
	}
	// Note: return the length of the input, not the number of
	// bytes printed by Fprintf, because this must behave like
	// an io.Writer.
	return len(msg), nil
}

func (n *netConn) writeString(p Priority, hostname, tag, msg, nl string) error {
	if n.local {
		// Compared to the network form below, the changes are:
		//	1. Use time.Stamp instead of time.RFC3339.
		//	2. Drop the hostname field from the Fprintf.
		timestamp := time.Now().Format(time.Stamp)
		_, err := fmt.Fprintf(n.conn, "<%d>%s %s[%d]: %s%s",
			p, timestamp,
			tag, os.Getpid(), msg, nl)
		return err
	}
	timestamp := time.Now().Format(time.RFC3339)
	_, err := fmt.Fprintf(n.conn, "<%d>%s %s %s[%d]: %s%s",
		p, timestamp, hostname,
		tag, os.Getpid(), msg, nl)
	return err
}

func (n *netConn) close() error {
	return n.conn.Close()
}

// NewLogger 创建 log.Logger，其指定优先级（syslog 设备和严重程度的组合）的输出被写到系统日志服务。
// 参数 logFlag 是传递给 log.New 的标志，用于创建 Logger。
func NewLogger(p Priority, logFlag int) (*log.Logger, error) {
	s, err := New(p, "")
	if err != nil {
		return nil, err
	}
	return log.New(s, "", logFlag), nil
}
