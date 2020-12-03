// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import "errors"

// Ticker 保管一个通道，并每隔一段时间向其传递"tick"。
type Ticker struct {
	C <-chan Time // 传递 tick 的管道。
	r runtimeTimer
}

// NewTicker 返回一个新的 Ticker，该 Ticker 包含一个通道字段，并会每隔时间段d就向该通道发送当时的时间。
// 它会调整时间间隔或者丢弃 tick 信息以适应反应慢的接收者。
// 参数 d Duration 必须大于等于 0，否则 NewTicker 将会 panic。
// 关闭该 Ticker 可以释放相关资源。
func NewTicker(d Duration) *Ticker {
	if d <= 0 {
		panic(errors.New("non-positive interval for NewTicker"))
	}
	// 给 channel 一个元素的时间缓冲区。
	// 如果 client 读取缓慢, 我们将丢弃 ticker 直到 client 赶上来。
	c := make(chan Time, 1)
	t := &Ticker{
		C: c,
		r: runtimeTimer{
			when:   when(d),
			period: int64(d),
			f:      sendTime,
			arg:    c,
		},
	}
	startTimer(&t.r)
	return t
}

// Stop 关闭一个 ticker。 After Stop, no more ticks will be sent.
// Stop 不会关闭 channel, 以防止并发的 goroutine 从 channel 中读取到错误的 tick。
func (t *Ticker) Stop() {
	stopTimer(&t.r)
}

// Reset stops a ticker and resets its period to the specified duration.
// The next tick will arrive after the new period elapses.
func (t *Ticker) Reset(d Duration) {
	if t.r.f == nil {
		panic("time: Reset called on uninitialized Ticker")
	}
	modTimer(&t.r, when(d), int64(d), t.r.f, t.r.arg, t.r.seq)
}

// Tick 是 NewTicker 的封装，只提供对 Ticker 的通道的访问。
// 虽然 Tick 对于不需要关闭的 Ticker 很好用，但是请注意，如果没有关闭 Ticker 的方法，GC 将不会回收底层的 Ticker 而产生"泄漏"。
//
// 如果参数 d Duration 小于等于 0，则 Tick 将返回 nil。
func Tick(d Duration) <-chan Time {
	if d <= 0 {
		return nil
	}
	return NewTicker(d).C
}
