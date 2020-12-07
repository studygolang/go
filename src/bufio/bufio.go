// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// bufio 包实现了有缓冲的 I/O 。它包装一个 io.Reader 或 io.Writer 接口对象，创建另一个也实现了该接口，且同时还提供了缓冲和一些文本 I/O 的帮助函数的对象。
package bufio

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	defaultBufSize = 4096
)

var (
	ErrInvalidUnreadByte = errors.New("bufio: invalid use of UnreadByte")
	ErrInvalidUnreadRune = errors.New("bufio: invalid use of UnreadRune")
	ErrBufferFull        = errors.New("bufio: buffer full")
	ErrNegativeCount     = errors.New("bufio: negative count")
)

// 带缓冲的输入

// Reader 实现了给一个 io.Reader 接口对象附加缓冲
type Reader struct {
	buf          []byte
	rd           io.Reader // reader provided by the client
	r, w         int       // buf read and write positions
	err          error
	lastByte     int // last byte read for UnreadByte; -1 means invalid
	lastRuneSize int // size of last rune read for UnreadRune; -1 means invalid
}

const minReadBufferSize = 16
const maxConsecutiveEmptyReads = 100

// NewReaderSize创建一个具有最少有 size 尺寸的缓冲、从 rd 读取的 *Reader。如果参数 rd 已经是一个具有足够大缓冲的 *Reader 类型值，会返回 rd。
func NewReaderSize(rd io.Reader, size int) *Reader {
	// Is it already a Reader?
	b, ok := rd.(*Reader)
	if ok && len(b.buf) >= size {
		return b
	}
	if size < minReadBufferSize {
		size = minReadBufferSize
	}
	r := new(Reader)
	r.reset(make([]byte, size), rd)
	return r
}

// NewReader 创建一个具有默认大小缓冲、从 rd 读取的 *Reader。
func NewReader(rd io.Reader) *Reader {
	return NewReaderSize(rd, defaultBufSize)
}

// Size 返回底层 buffer 的字节数
func (b *Reader) Size() int { return len(b.buf) }

// Reset 丢弃所有缓冲中的数据，重置所有状态，并且内部 buf 转为从 r 中读取数据
func (b *Reader) Reset(r io.Reader) {
	b.reset(b.buf, r)
}

func (b *Reader) reset(buf []byte, r io.Reader) {
	*b = Reader{
		buf:          buf,
		rd:           r,
		lastByte:     -1,
		lastRuneSize: -1,
	}
}

var errNegativeRead = errors.New("bufio: reader returned negative count from Read")

// fill 读取一块新的数据到 buffer 中
func (b *Reader) fill() {
	// Slide existing data to beginning.
	if b.r > 0 {
		copy(b.buf, b.buf[b.r:b.w])
		b.w -= b.r
		b.r = 0
	}

	if b.w >= len(b.buf) {
		panic("bufio: tried to fill full buffer")
	}

	// Read new data: try a limited number of times.
	for i := maxConsecutiveEmptyReads; i > 0; i-- {
		n, err := b.rd.Read(b.buf[b.w:])
		if n < 0 {
			panic(errNegativeRead)
		}
		b.w += n
		if err != nil {
			b.err = err
			return
		}
		if n > 0 {
			return
		}
	}
	b.err = io.ErrNoProgress
}

func (b *Reader) readErr() error {
	err := b.err
	b.err = nil
	return err
}

// Peek返回输入流的下 n 个字节，而不会移动读取位置。返回的 []byte 只在下一次调用读取操作前合法。
// 如果Peek返回的切片长度比 n 小，它也会返会一个错误说明原因。如果 n 比缓冲尺寸还大，将返回错误 ErrBufferFull。
//
// 调用 Peek 会导致 UnreadByte 或 UnreadRune 在下一次read操作前调用失败
func (b *Reader) Peek(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrNegativeCount
	}

	b.lastByte = -1
	b.lastRuneSize = -1

	for b.w-b.r < n && b.w-b.r < len(b.buf) && b.err == nil {
		b.fill() // b.w-b.r < len(b.buf) => buffer is not full
	}

	if n > len(b.buf) {
		return b.buf[b.r:b.w], ErrBufferFull
	}

	// 0 <= n <= len(b.buf)
	var err error
	if avail := b.w - b.r; avail < n {
		// not enough data in buffer
		n = avail
		err = b.readErr()
		if err == nil {
			err = ErrBufferFull
		}
	}
	return b.buf[b.r : b.r+n], err
}


// Discard 会跳过 n 个字节，并且返回丢弃的字节数
//
// 如果 Discard 跳过了小于n个字节，那么会返回一个错误
// 如果 n 在[0, b.Buffeded()]区间，那么 Discard 一定可以成功返回，此时不会从底层 io.Reader 中去读取数据
func (b *Reader) Discard(n int) (discarded int, err error) {
	if n < 0 {
		return 0, ErrNegativeCount
	}
	if n == 0 {
		return
	}
	remain := n
	for {
		skip := b.Buffered()
		if skip == 0 {
			b.fill()
			skip = b.Buffered()
		}
		if skip > remain {
			skip = remain
		}
		b.r += skip
		remain -= skip
		if remain == 0 {
			return n, nil
		}
		if b.err != nil {
			return n - remain, b.readErr()
		}
	}
}

// Read 读取数据到p中
// 本方法返回读入p中的字节数。
// 一次取走的数据几乎是一次从底层Reader中独居的数据量，故而，n 可能会小于 len(p)
// 如果需要读取刚好 len(p) 个字节，请调用 io.ReadFull(b,p) 方法。
// 如果刚好读到文件结尾(EOF), 那么返回的 n 将为 0, 并且 err 是 io.EOF
func (b *Reader) Read(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 {
		if b.Buffered() > 0 {
			return 0, nil
		}
		return 0, b.readErr()
	}
	if b.r == b.w {
		if b.err != nil {
			return 0, b.readErr()
		}
		if len(p) >= len(b.buf) {
			// Large read, empty buffer.
			// Read directly into p to avoid copy.
			n, b.err = b.rd.Read(p)
			if n < 0 {
				panic(errNegativeRead)
			}
			if n > 0 {
				b.lastByte = int(p[n-1])
				b.lastRuneSize = -1
			}
			return n, b.readErr()
		}
		// One read.
		// Do not use b.fill, which will loop.
		b.r = 0
		b.w = 0
		n, b.err = b.rd.Read(b.buf)
		if n < 0 {
			panic(errNegativeRead)
		}
		if n == 0 {
			return 0, b.readErr()
		}
		b.w += n
	}

	// copy as much as we can
	n = copy(p, b.buf[b.r:b.w])
	b.r += n
	b.lastByte = int(b.buf[b.r-1])
	b.lastRuneSize = -1
	return n, nil
}

// ReadByte读 取并返回一个字节。
// 如果读取不到数据，将返回 error
func (b *Reader) ReadByte() (byte, error) {
	b.lastRuneSize = -1
	for b.r == b.w {
		if b.err != nil {
			return 0, b.readErr()
		}
		b.fill() // buffer is empty
	}
	c := b.buf[b.r]
	b.r++
	b.lastByte = int(c)
	return c, nil
}

// UnreadByte 将上一个已经读取的字节置为未读。 只有最近读取的那个字节可以变为 unread。
// 如果上一个函数调用不是 read 操作，那么 UnreadByte 将返回 error。特别注意，Peek 不算做 read 操作。
func (b *Reader) UnreadByte() error {
	if b.lastByte < 0 || b.r == 0 && b.w > 0 {
		return ErrInvalidUnreadByte
	}
	// b.r > 0 || b.w == 0
	if b.r > 0 {
		b.r--
	} else {
		// b.r == 0 && b.w == 0
		b.w = 1
	}
	b.buf[b.r] = byte(b.lastByte)
	b.lastByte = -1
	b.lastRuneSize = -1
	return nil
}

// ReadRune 读取单个 UTF-8 编码字符并且返回这个字符和这个字符的字节数。
// 如果这个已编码字符不合法，那么将只消费一个字节，并返回 unicode.ReplacementChar (U+FFFD),返回的 size 的值为 1。
func (b *Reader) ReadRune() (r rune, size int, err error) {
	for b.r+utf8.UTFMax > b.w && !utf8.FullRune(b.buf[b.r:b.w]) && b.err == nil && b.w-b.r < len(b.buf) {
		b.fill() // b.w-b.r < len(buf) => buffer is not full
	}
	b.lastRuneSize = -1
	if b.r == b.w {
		return 0, 0, b.readErr()
	}
	r, size = rune(b.buf[b.r]), 1
	if r >= utf8.RuneSelf {
		r, size = utf8.DecodeRune(b.buf[b.r:b.w])
	}
	b.r += size
	b.lastByte = int(b.buf[b.r-1])
	b.lastRuneSize = size
	return r, size, nil
}

// UnreadRune 将上一个已经读取的 UTF-8 编码字符置为未读。 只有最近读取的那个字节可以变为 unread。
// 如果上一个函数调用不是 read 操作，那么 UnreadRune 将返回 error。(从这里方面来看， 本方法 比UnreadByte 更严格。)
func (b *Reader) UnreadRune() error {
	if b.lastRuneSize < 0 || b.r < b.lastRuneSize {
		return ErrInvalidUnreadRune
	}
	b.r -= b.lastRuneSize
	b.lastByte = -1
	b.lastRuneSize = -1
	return nil
}

// Buffered 返回当前 buffer 中可以读取的字节数
func (b *Reader) Buffered() int { return b.w - b.r }

// ReadSlice 会一直读取直到遇到指定的字节delim,返回一个指向当前 buffer 的一个 slice。
// 数据在下一次读取操作前是合法的。
// 如果 ReadSlice 在读取到 delim 之前遇到了错误， 则会返回buffer中所有数据和该错误(一般会是 io.EOF)。
// 如果这个 buffer 填充过程中一直没有遇到 delim， ReadSlic e会返回错误 ErrBufferFull。
// 由于通过 ReadSlice 中的数据在下一次 I/O 操作中会被覆盖，客户端最好是使用 ReadBytes 或者 ReadString 方法。
// 当且仅当读取一行结束仍无法找到 delim 字节时会返回一个非空 err。
func (b *Reader) ReadSlice(delim byte) (line []byte, err error) {
	s := 0 // search start index
	for {
		// Search buffer.
		if i := bytes.IndexByte(b.buf[b.r+s:b.w], delim); i >= 0 {
			i += s
			line = b.buf[b.r : b.r+i+1]
			b.r += i + 1
			break
		}

		// Pending error?
		if b.err != nil {
			line = b.buf[b.r:b.w]
			b.r = b.w
			err = b.readErr()
			break
		}

		// Buffer full?
		if b.Buffered() >= len(b.buf) {
			b.r = b.w
			line = b.buf
			err = ErrBufferFull
			break
		}

		s = b.w - b.r // do not rescan area we scanned before

		b.fill() // buffer is not full
	}

	// Handle last byte, if any.
	if i := len(line) - 1; i >= 0 {
		b.lastByte = int(line[i])
		b.lastRuneSize = -1
	}

	return
}

// ReadLine 是一个底层的行读原语。推荐调用者用 ReadBytes('\n')，ReadString('\n') 或者 Scanner 来调用。
//
// ReadLine尝试返回一行数据，但是不包括行结束符的那个字节。
// 如果该行数据对 buffer 来说太长的话，将会设置 isPrefix 并且返回前面部分的数据。 这一行剩下的数据会在
// 下一次请求中返回。 当这一行最后一部分的数据返回时 isPrefix 会变为 false。 返回的数据只在下一次调用 readLine
// 之前有效。ReadLine 要么返回一个非空的行数据，要不返回 error， 但两者不会同时出现。
//
// ReadLine 返回的数据中不会包含行结束符("\r\n", "\n")。 如果输入没有行结束符也不会有任何的报错或提醒。
// 在 ReadLine 之后调用 UnreadByte 将总是将最后一个字节置为未读(很可能是行结束符)，即使这个字节不在 ReadLine 的返回数据中

func (b *Reader) ReadLine() (line []byte, isPrefix bool, err error) {
	line, err = b.ReadSlice('\n')
	if err == ErrBufferFull {
		// Handle the case where "\r\n" straddles the buffer.
		if len(line) > 0 && line[len(line)-1] == '\r' {
			// Put the '\r' back on buf and drop it from line.
			// Let the next call to ReadLine check for "\r\n".
			if b.r == 0 {
				// should be unreachable
				panic("bufio: tried to rewind past start of buffer")
			}
			b.r--
			line = line[:len(line)-1]
		}
		return line, true, nil
	}

	if len(line) == 0 {
		if err != nil {
			line = nil
		}
		return
	}
	err = nil

	if line[len(line)-1] == '\n' {
		drop := 1
		if len(line) > 1 && line[len(line)-2] == '\r' {
			drop = 2
		}
		line = line[:len(line)-drop]
	}
	return
}

// collectFragments 会一直读取数据直到遇到了第一个指定的结束字节 delim。 
// 返回 (buffers 的 slice, 在 delim 前剩余的字节, 前两部分的总字节数， error)。
// 这个完整的结果和 `bytes.Join(append(fullBuffers, finalFragment), nil)` 相同， 这个结果的形式主要是方便调用者最小化调用过程中的内存分配和复制。
func (b *Reader) collectFragments(delim byte) (fullBuffers [][]byte, finalFragment []byte, totalLen int, err error) {
	var frag []byte
	// Use ReadSlice to look for delim, accumulating full buffers.
	for {
		var e error
		frag, e = b.ReadSlice(delim)
		if e == nil { // got final fragment
			break
		}
		if e != ErrBufferFull { // unexpected error
			err = e
			break
		}

		// Make a copy of the buffer.
		buf := make([]byte, len(frag))
		copy(buf, frag)
		fullBuffers = append(fullBuffers, buf)
		totalLen += len(buf)
	}

	totalLen += len(frag)
	return fullBuffers, frag, totalLen, err
}

// ReadBytes 会一直读取数据直到遇到了第一个指定的结束字节 delim，返回一个包含了 delim 的 slice。
// 如果 ReadBytes 在找到 delim 之前遇到了 error, 他将会返回遇到 error 前的 buffer 数据 和 error 本身（通常会是 io.EOF）。
// 当且仅当返回的数据不是以 delim 结尾时， ReadBytes 会返回不为 nil 的 err。
// 对于简单的使用， 使用 Scanner 可能会更加方便。
func (b *Reader) ReadBytes(delim byte) ([]byte, error) {
	full, frag, n, err := b.collectFragments(delim)
	// Allocate new buffer to hold the full pieces and the fragment.
	buf := make([]byte, n)
	n = 0
	// Copy full pieces and fragment in.
	for i := range full {
		n += copy(buf[n:], full[i])
	}
	copy(buf[n:], frag)
	return buf, err
}

// ReadString 会一直读取数据直到遇到了第一个指定的结束字节 delim，返回一个包含了 delim 的 string
// 如果 ReadString 在找到 delim 之前遇到了 error, 他将会返回遇到 error 前的 数据 和 error 本身（通常会是 io.EOF）。
// 当且仅当返回的数据不是以 delim 结尾时， ReadBytes 会返回不为 nil 的 err。
// 对于简单的使用， 使用 Scanner 可能会更加方便。
func (b *Reader) ReadString(delim byte) (string, error) {
	full, frag, n, err := b.collectFragments(delim)
	// Allocate new buffer to hold the full pieces and the fragment.
	var buf strings.Builder
	buf.Grow(n)
	// Copy full pieces and fragment in.
	for _, fb := range full {
		buf.Write(fb)
	}
	buf.Write(frag)
	return buf.String(), err
}

// WriteTo 实现了 io.WriterTo
// 这可能会是多次调用底层 Reader 的 Read 方法。
// 如果底层的 reader 支持 WriteTo 方法， 那么将直接调用底层的 WriteTo 方法，而不是使用 buffer做缓冲。
func (b *Reader) WriteTo(w io.Writer) (n int64, err error) {
	n, err = b.writeBuf(w)
	if err != nil {
		return
	}

	if r, ok := b.rd.(io.WriterTo); ok {
		m, err := r.WriteTo(w)
		n += m
		return n, err
	}

	if w, ok := w.(io.ReaderFrom); ok {
		m, err := w.ReadFrom(b.rd)
		n += m
		return n, err
	}

	if b.w-b.r < len(b.buf) {
		b.fill() // buffer not full
	}

	for b.r < b.w {
		// b.r < b.w => buffer is not empty
		m, err := b.writeBuf(w)
		n += m
		if err != nil {
			return n, err
		}
		b.fill() // buffer is empty
	}

	if b.err == io.EOF {
		b.err = nil
	}

	return n, b.readErr()
}

var errNegativeWrite = errors.New("bufio: writer returned negative count from Write")

// writeBuf 将 Reader 的 buffer 数据 写到 writer 中去。
func (b *Reader) writeBuf(w io.Writer) (int64, error) {
	n, err := w.Write(b.buf[b.r:b.w])
	if n < 0 {
		panic(errNegativeWrite)
	}
	b.r += n
	return int64(n), err
}


// 带缓冲的输出

// Writer 为底层的 io.Writer 对象实现了缓冲。
// 如果在写的过程中发生了错误， 那么将不在接收数据以及接下来的所有写操作 (write), 刷新 (flush)， 并且会返回error。
// 当所有的数据都写完之后， 调用者应当调用 Flush 方法来确保所有的数据都已经写入到了底层的 io.Writer中去了。

type Writer struct {
	err error
	buf []byte
	n   int
	wr  io.Writer
}

// NewWriterSize 返回了一个新的 Writer，这个 Writer 的buffer 至少为制定的 size 大小。
// 如果这个参数 io.Writer 已经是一个带有足够大 size 的 Writer， 那么将返回底层的 Writer。
func NewWriterSize(w io.Writer, size int) *Writer {
	// Is it already a Writer?
	b, ok := w.(*Writer)
	if ok && len(b.buf) >= size {
		return b
	}
	if size <= 0 {
		size = defaultBufSize
	}
	return &Writer{
		buf: make([]byte, size),
		wr:  w,
	}
}

// NewWriter 返回一个带默认 size 的 buffer 的Writer。
func NewWriter(w io.Writer) *Writer {
	return NewWriterSize(w, defaultBufSize)
}

// Size 返回底层buffer的字节数。
func (b *Writer) Size() int { return len(b.buf) }

// Reset 丢弃所有未被刷新的 buffer 数据， 清除所有error, 并重置 b 的输出到 w 中去。
func (b *Writer) Reset(w io.Writer) {
	b.err = nil
	b.n = 0
	b.wr = w
}

// Flush 将 buffer 中的数据写入到底层的 io.Writer中去。
func (b *Writer) Flush() error {
	if b.err != nil {
		return b.err
	}
	if b.n == 0 {
		return nil
	}
	n, err := b.wr.Write(b.buf[0:b.n])
	if n < b.n && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		if n > 0 && n < b.n {
			copy(b.buf[0:b.n-n], b.buf[n:b.n])
		}
		b.n -= n
		b.err = err
		return err
	}
	b.n = 0
	return nil
}

// Available 返回在 buffer 中有尚未使用的字节数。
func (b *Writer) Available() int { return len(b.buf) - b.n }

// Buffered 返回 写入到 buffer 中的字节数
func (b *Writer) Buffered() int { return b.n }

// Write 将 p 中的数据写入到 buffer。
// 返回写入的字节数。
// 如果 nn< len(p)，那么将会返回一个 error 解释为什么写入的数据少了。
func (b *Writer) Write(p []byte) (nn int, err error) {
	for len(p) > b.Available() && b.err == nil {
		var n int
		if b.Buffered() == 0 {
			// Large write, empty buffer.
			// Write directly from p to avoid copy.
			n, b.err = b.wr.Write(p)
		} else {
			n = copy(b.buf[b.n:], p)
			b.n += n
			b.Flush()
		}
		nn += n
		p = p[n:]
	}
	if b.err != nil {
		return nn, b.err
	}
	n := copy(b.buf[b.n:], p)
	b.n += n
	nn += n
	return nn, nil
}

// WriteByte 写入单个byte。
func (b *Writer) WriteByte(c byte) error {
	if b.err != nil {
		return b.err
	}
	if b.Available() <= 0 && b.Flush() != nil {
		return b.err
	}
	b.buf[b.n] = c
	b.n++
	return nil
}

// WriteRune 写入单个的 Unicode 码， 返回写入的字节数和 error。
func (b *Writer) WriteRune(r rune) (size int, err error) {
	if r < utf8.RuneSelf {
		err = b.WriteByte(byte(r))
		if err != nil {
			return 0, err
		}
		return 1, nil
	}
	if b.err != nil {
		return 0, b.err
	}
	n := b.Available()
	if n < utf8.UTFMax {
		if b.Flush(); b.err != nil {
			return 0, b.err
		}
		n = b.Available()
		if n < utf8.UTFMax {
			// Can only happen if buffer is silly small.
			return b.WriteString(string(r))
		}
	}
	size = utf8.EncodeRune(b.buf[b.n:], r)
	b.n += size
	return size, nil
}

// WriteString 写入一个string。
// 返回写入的字节数。
// 如果返回的字节数 比 len(s) 小， 返回 error 揭示为什么写入的数据少了。
func (b *Writer) WriteString(s string) (int, error) {
	nn := 0
	for len(s) > b.Available() && b.err == nil {
		n := copy(b.buf[b.n:], s)
		b.n += n
		nn += n
		s = s[n:]
		b.Flush()
	}
	if b.err != nil {
		return nn, b.err
	}
	n := copy(b.buf[b.n:], s)
	b.n += n
	nn += n
	return nn, nil
}

// ReadFrom 实现了 io.ReaderFrom。 如果底层的 writer 支持 这个 ReadFrom 方法，并且 b 还没有缓冲数据，
// 那么会直接调用底层的 ReadFrom 方法 而不是写入到buffer中。
func (b *Writer) ReadFrom(r io.Reader) (n int64, err error) {
	if b.err != nil {
		return 0, b.err
	}
	if b.Buffered() == 0 {
		if w, ok := b.wr.(io.ReaderFrom); ok {
			n, err = w.ReadFrom(r)
			b.err = err
			return n, err
		}
	}
	var m int
	for {
		if b.Available() == 0 {
			if err1 := b.Flush(); err1 != nil {
				return n, err1
			}
		}
		nr := 0
		for nr < maxConsecutiveEmptyReads {
			m, err = r.Read(b.buf[b.n:])
			if m != 0 || err != nil {
				break
			}
			nr++
		}
		if nr == maxConsecutiveEmptyReads {
			return n, io.ErrNoProgress
		}
		b.n += m
		n += int64(m)
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		// If we filled the buffer exactly, flush preemptively.
		if b.Available() == 0 {
			err = b.Flush()
		} else {
			err = nil
		}
	}
	return n, err
}

// 带缓冲的输入输出

// ReadWriter 保存了一个 Reader 和一个 Writer 的指针。
// 实现了 io.ReadWriter接口。
type ReadWriter struct {
	*Reader
	*Writer
}

// NewReadWriter 申请了一个新的 ReaderWriter 来调用 r， w。
func NewReadWriter(r *Reader, w *Writer) *ReadWriter {
	return &ReadWriter{r, w}
}
