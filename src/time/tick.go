// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import "errors"

// A Ticker holds a channel that delivers `ticks' of a clock
// at intervals.
type Ticker struct {
	C <-chan Time // The channel on which the ticks are delivered.
	r runtimeTimer
}

// NewTicker returns a new Ticker containing a channel that will send the
// time with a period specified by the duration argument.
// It adjusts the intervals or drops ticks to make up for slow receivers.
// The duration d must be greater than zero; if not, NewTicker will panic.
// Stop the ticker to release associated resources.
func NewTicker(d Duration) *Ticker {
	if d <= 0 {
		panic(errors.New("non-positive interval for NewTicker"))
	}
	// Give the channel a 1-element time buffer.
	// If the client falls behind while reading, we drop ticks
	// on the floor until the client catches up.
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

// Stop turns off a ticker. After Stop, no more ticks will be sent.
// Stop does not close the channel, to prevent a concurrent goroutine
// reading from the channel from seeing an erroneous "tick".
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
