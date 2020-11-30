// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytes

// 用于编组数据的简单字节缓冲区。

import (
	"errors"
	"io"
	"unicode/utf8"
)

// smallBufferSize 是初始分配的最小容量。
const smallBufferSize = 64

// Buffer 是具有读和写方法的可变大小的字节缓冲区。
// Buffer 的零值是准备使用的空缓冲区。
type Buffer struct {
	buf      []byte // 内容是字节 buf[off : len(buf)]
	off      int    // 在 &buf[off] 处读, 在 &buf[len(buf)] 处写
	lastRead readOp // 最后的读取操作, 使 Unread* 可以正常工作。
}

// readOp 常数描述在缓冲区上执行的最后操作，
// 以便 UnreadRune 和 UnreadByte 可以检查是否使用无效。
// 选择 opReadRuneX 常数以便转换成对应所读的 rune 大小的 int。
type readOp int8

// 不要使用 iota，因为值需要与名称和注释对应，在显示时更易于查看。
const (
	opRead      readOp = -1 // 任何其他读取操作。
	opInvalid   readOp = 0  // 非读取操作。
	opReadRune1 readOp = 1  // 读取大小为 1 的 rune。
	opReadRune2 readOp = 2  // 读取大小为 2 的 rune。
	opReadRune3 readOp = 3  // 读取大小为 3 的 rune。
	opReadRune4 readOp = 4  // 读取大小为 4 的 rune。
)

// 如果无法分配内存，将数据存储在缓冲区中，则会将 ErrTooLarge 传递为 panic。
var ErrTooLarge = errors.New("bytes.Buffer: too large")
var errNegativeRead = errors.New("bytes.Buffer: reader returned negative count from Read")

const maxInt = int(^uint(0) >> 1)

// Bytes 返回长度为 b.Len() 的切片，其中包含缓冲区的未读部分。
// 切片仅在下一次缓冲区修改之前有效（即直到下一次调用诸如 Read，Write，Reset 或 Truncate之类的方法为止。）
// 至少在下一次修改缓冲区之前，切片会为缓冲区内容起别名，
// 因此，直接对切片进行更改将影响将来读取的结果。
func (b *Buffer) Bytes() []byte { return b.buf[b.off:] }

// String 以字符串形式返回 buffer 未读部分的内容
// 如果 Buffer 是 nil 指针，则返回 "<nil>"。
//
// 欲更有效地构建字符串，请参见 strings.Builder 类型。
func (b *Buffer) String() string {
	if b == nil {
		// 特殊情况，在调试中很有用。
		return "<nil>"
	}
	return string(b.buf[b.off:])
}

// empty 判断缓冲区的未读部分是否为空。
func (b *Buffer) empty() bool { return len(b.buf) <= b.off }

// Len 返回缓冲区未读部分的字节数；
// b.Len() == len(b.Bytes()).
func (b *Buffer) Len() int { return len(b.buf) - b.off }

// Cap 返回缓冲区基础字节片的容量，即为缓冲区数据分配的总空间。
func (b *Buffer) Cap() int { return cap(b.buf) }

// Truncate 将丢弃缓冲区中除前 n 个未读字节外的所有字节，
// 仍继续使用已分配的相同存储空间。
// 如果 n 为负数或大于缓冲区的长度，则会 panic。
func (b *Buffer) Truncate(n int) {
	if n == 0 {
		b.Reset()
		return
	}
	b.lastRead = opInvalid
	if n < 0 || n > b.Len() {
		panic("bytes.Buffer: truncation out of range")
	}
	b.buf = b.buf[:b.off+n]
}

// Reset 会将缓冲区重置为空，
// 但它保留了底层存储供以后的写操作使用。
// Reset 与 Truncate(0) 相同。
func (b *Buffer) Reset() {
	b.buf = b.buf[:0]
	b.off = 0
	b.lastRead = opInvalid
}

// tryGrowByReslice 是 grow 的内联版本，用于快速案例，其中
// 内部缓冲区只需要被切片。
// 它返回应该在其中写入字节的索引以及索引是否成功。
func (b *Buffer) tryGrowByReslice(n int) (int, bool) {
	if l := len(b.buf); n <= cap(b.buf)-l {
		b.buf = b.buf[:l+n]
		return l, true
	}
	return 0, false
}

// grow 增长缓冲区以保证更多 n 个字节有空间。
// 它返回要在其中写入字节的索引。
// 如果缓冲区无法增长，则会因 ErrTooLarge 而 panic。
func (b *Buffer) grow(n int) int {
	m := b.Len()
	// 如果缓冲区为空，重置以恢复空间。
	if m == 0 && b.off != 0 {
		b.Reset()
	}
	// 尝试通过切片来增长。
	if i, ok := b.tryGrowByReslice(n); ok {
		return i
	}
	if b.buf == nil && n <= smallBufferSize {
		b.buf = make([]byte, n, smallBufferSize)
		return 0
	}
	c := cap(b.buf)
	if n <= c/2-m {
		// 我们可以向下滑动，而不必分配新的切片。
		// 我们只需要 m+n <= c 即可滑动，
		// 将容量改为增加两倍，以避免花费所有的时间进行复制。
		copy(b.buf, b.buf[b.off:])
	} else if c > maxInt-c-n {
		panic(ErrTooLarge)
	} else {
		// 没有足够的空间，需要分配。
		buf := makeSlice(2*c + n)
		copy(buf, b.buf[b.off:])
		b.buf = buf
	}
	// 恢复 b.off 和 len(b.buf).
	b.off = 0
	b.buf = b.buf[:m+n]
	return m
}

// 如有必要，Grow 可以增加缓冲区的容量, 以保证有足够的空间用于另外的 n 个字节。
// 在 Grow(n) 后， 至少有 n 个字节可以写入到没有分配的缓冲区。
// 如果 n 为负数, Grow 将会 panic。
// 如果缓冲区无法增长，则会因 ErrTooLarge 而 panic。
func (b *Buffer) Grow(n int) {
	if n < 0 {
		panic("bytes.Buffer.Grow: negative count")
	}
	m := b.grow(n)
	b.buf = b.buf[:m]
}

// Write 将 p 的内容追加到缓冲区，根据需要增加缓冲区。
// 返回值 n 是 p 的长度；err 总是 nil。
// 如果缓冲区太大，Write 会因 ErrTooLarge 而 panic。
func (b *Buffer) Write(p []byte) (n int, err error) {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(len(p))
	if !ok {
		m = b.grow(len(p))
	}
	return copy(b.buf[m:], p), nil
}

// WriteString 将 s 的内容追加到缓冲区，根据需要增加缓冲区。
// 返回值 n 是 s 的长度；err 总是 nil。
// 如果缓冲区太大，WriteString 会因 ErrTooLarge 而 panic。
func (b *Buffer) WriteString(s string) (n int, err error) {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(len(s))
	if !ok {
		m = b.grow(len(s))
	}
	return copy(b.buf[m:], s), nil
}

// MinRead 是 Buffer.ReadFrom 传递给 Read 调用的最小切片大小。
// 只要 Buffer 有至少 MinRead 个字节，超过容纳 r 的内容所需的字节，
// ReadFrom 不会增长底层缓冲区。
const MinRead = 512

// ReadFrom 从 r 读取数据直到 EOF 并将其追加到缓冲区，根据需要增加缓冲区。
// 返回值 n 是读取的字节数。
// 读取过程中遇到的除 io.EOF 外的任何错误会被返回。
// 如果缓冲区太大，ReadFrom 会因 ErrTooLarge 而 panic。
func (b *Buffer) ReadFrom(r io.Reader) (n int64, err error) {
	b.lastRead = opInvalid
	for {
		i := b.grow(MinRead)
		b.buf = b.buf[:i]
		m, e := r.Read(b.buf[i:cap(b.buf)])
		if m < 0 {
			panic(errNegativeRead)
		}

		b.buf = b.buf[:i+m]
		n += int64(m)
		if e == io.EOF {
			return n, nil // e 为 EOF，因此显式返回 nil
		}
		if e != nil {
			return n, e
		}
	}
}

// makeSlice 分配大小为 n 的切片。如果分配失败，则会因 ErrTooLarge 而 panic。
func makeSlice(n int) []byte {
	// 如果失败，则给出一个已知错误。
	defer func() {
		if recover() != nil {
			panic(ErrTooLarge)
		}
	}()
	return make([]byte, n)
}

// WriteTo 将数据写入 w，直到缓冲区耗尽或发生错误。
// 返回值 n 是写入的字节数；它总是适合 int，
// 但与 io.WriterTo 接口匹配的是 int64。
// 写入过程中遇到的任何错误也会返回。
func (b *Buffer) WriteTo(w io.Writer) (n int64, err error) {
	b.lastRead = opInvalid
	if nBytes := b.Len(); nBytes > 0 {
		m, e := w.Write(b.buf[b.off:])
		if m > nBytes {
			panic("bytes.Buffer.WriteTo: invalid Write count")
		}
		b.off += m
		n = int64(m)
		if e != nil {
			return n, e
		}
		// 根据 io.Writer 中的 Write 方法的定义，
		// 所有字节都应该已经被写入
		if m != nBytes {
			return n, io.ErrShortWrite
		}
	}
	// 缓冲区现在为空；重启。
	b.Reset()
	return n, nil
}

// WriteByte 将字节 c 追加到缓冲区，根据需要增大缓冲区。
// 返回的错误始终为 nil，但包含它是为了匹配 bufio.Writer 的 WriteByte。
// 如果缓冲区太大，WriteByte 会因 ErrTooLarge 而 panic。
func (b *Buffer) WriteByte(c byte) error {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(1)
	if !ok {
		m = b.grow(1)
	}
	b.buf[m] = c
	return nil
}

// WriteRune 将 Unicode 代码点 r 的 UTF-8 编码追加到缓冲区，
// 返回其长度和错误，该错误始终为 nil，
// 包含它是为了匹配 bufio.Writer 的 WriteRune。缓冲区根据需要增长；
//如果太大，WriteRune 会因 ErrTooLarge 而 panic。
func (b *Buffer) WriteRune(r rune) (n int, err error) {
	if r < utf8.RuneSelf {
		b.WriteByte(byte(r))
		return 1, nil
	}
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(utf8.UTFMax)
	if !ok {
		m = b.grow(utf8.UTFMax)
	}
	n = utf8.EncodeRune(b.buf[m:m+utf8.UTFMax], r)
	b.buf = b.buf[:m+n]
	return n, nil
}

// Read 从缓冲区读取下一个 p 长度的字节，直到缓冲区耗尽。
// 返回值 n 是读取的字节数。
// 如果缓冲区没有要返回的数据，err 为 io.EOF（除非 len(p) 为零）；
// 否则为 nil。
func (b *Buffer) Read(p []byte) (n int, err error) {
	b.lastRead = opInvalid
	if b.empty() {
		// Buffer is empty, reset to recover space.
		b.Reset()
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	n = copy(p, b.buf[b.off:])
	b.off += n
	if n > 0 {
		b.lastRead = opRead
	}
	return n, nil
}

// Next 返回一个切片，其中包含缓冲区中的下n个字节，
// 向前推进缓冲区，就像字节已经被 Read 返回。
// 如果缓冲区中的字节数少于 n 个，则 Next 返回整个缓冲区。
// 切片仅在下一次调用 read 或 write 方法之前有效。
func (b *Buffer) Next(n int) []byte {
	b.lastRead = opInvalid
	m := b.Len()
	if n > m {
		n = m
	}
	data := b.buf[b.off : b.off+n]
	b.off += n
	if n > 0 {
		b.lastRead = opRead
	}
	return data
}

// ReadByte 读取并从缓冲区返回下一个字节。
// 如果没有可用的字节，则返回错误 io.EOF。
func (b *Buffer) ReadByte() (byte, error) {
	if b.empty() {
		// 缓冲区为空，重置以恢复空间。
		b.Reset()
		return 0, io.EOF
	}
	c := b.buf[b.off]
	b.off++
	b.lastRead = opRead
	return c, nil
}

// ReadRune 从缓冲区读取并返回下一个 UTF-8 编码的 Unicode 代码点。
// 如果没有可用的字节，则返回的错误是 io.EOF。
// 如果字节是错误的 UTF-8 编码，则它占用一个字节并返回 U+FFFD, 1。
func (b *Buffer) ReadRune() (r rune, size int, err error) {
	if b.empty() {
		// 缓冲区为空，重置以恢复空间。
		b.Reset()
		return 0, 0, io.EOF
	}
	c := b.buf[b.off]
	if c < utf8.RuneSelf {
		b.off++
		b.lastRead = opReadRune1
		return rune(c), 1, nil
	}
	r, n := utf8.DecodeRune(b.buf[b.off:])
	b.off += n
	b.lastRead = readOp(n)
	return r, n, nil
}

// UnreadRune 不读取从 ReadRune 返回的最后一个 rune。
// 如果缓冲区上最近的读操作或写操作没有成功，UnreadRune 返回一个错误。（在这方面
// 它比 UnreadByte 更严格，后者将不通过任何读取操作读取最后一个字节。）
func (b *Buffer) UnreadRune() error {
	if b.lastRead <= opInvalid {
		return errors.New("bytes.Buffer: UnreadRune: previous operation was not a successful ReadRune")
	}
	if b.off >= int(b.lastRead) {
		b.off -= int(b.lastRead)
	}
	b.lastRead = opInvalid
	return nil
}

var errUnreadByte = errors.New("bytes.Buffer: UnreadByte: previous operation was not a successful read")

// UnreadByte 解除读取最近成功读取操作返回的最后一个字节，至少一个字节。
// 如果写发生在上次读取之后，如果上次读取返回了错误，或者如果 read 读取了零字节，
// UnreadByte 返回一个错误。
func (b *Buffer) UnreadByte() error {
	if b.lastRead == opInvalid {
		return errUnreadByte
	}
	b.lastRead = opInvalid
	if b.off > 0 {
		b.off--
	}
	return nil
}

// ReadBytes 读取直到输入第一次出现 delim 为止，
// 返回一个切片，其中包含直到分隔符的数据。
// 如果 ReadBytes 在找到分隔符之前遇到错误，
// 它返回错误之前读取的数据和错误本身（通常为 io.EOF）。
// 当且仅当返回的数据不是以 delim 结尾，ReadBytes 返回 err != nil。
func (b *Buffer) ReadBytes(delim byte) (line []byte, err error) {
	slice, err := b.readSlice(delim)
	// 返回切片的副本。缓冲区的备份数组可能会被以后的调用覆盖。
	line = append(line, slice...)
	return line, err
}

// readSlice 类似于 ReadBytes，但它返回对内部缓冲区数据的引用。
func (b *Buffer) readSlice(delim byte) (line []byte, err error) {
	i := IndexByte(b.buf[b.off:], delim)
	end := b.off + i + 1
	if i < 0 {
		end = len(b.buf)
		err = io.EOF
	}
	line = b.buf[b.off:end]
	b.off = end
	b.lastRead = opRead
	return line, err
}

// ReadString 读取直到输入第一次出现 delim 为止，
// 返回一个字符串，其中包含直到分隔符的数据。
// 如果 ReadString 在找到分隔符之前遇到错误，
// 它返回错误之前读取的数据和错误本身（通常为 io.EOF）。
// 当且仅当返回的数据不是以 delim 结尾，ReadString 返回 err != nil。
func (b *Buffer) ReadString(delim byte) (line string, err error) {
	slice, err := b.readSlice(delim)
	return string(slice), err
}

// NewBuffer 使用 buf 作为初始内容创建并初始化一个新的 Buffer。
// 新的 Buffer 获得 buf 的所有权，调用者在此调用之后不应该使用 buf。
// NewBuffer 旨在准备一个缓冲区以读取现有数据。
// 它也可以用来设置用于写入的内部缓冲区的初始大小。
// 要做到这一点，buf 应该具备所需的容量，但长度为 0。
//
// 在大多数情况下，new(Buffer) (或者声明一个 Buffer 变量) 是足以初始化一个 Buffer 的。
func NewBuffer(buf []byte) *Buffer { return &Buffer{buf: buf} }

// NewBufferString 使用字符串 s 作为初始内容创建并初始化一个新的 Buffer。
// 目的是准备一个缓冲区以读取现有的字符串。
//
// 在大多数情况下，new(Buffer) （或者声明一个 Buffer 变量）是足以初始化一个 Buffer 的。
func NewBufferString(s string) *Buffer {
	return &Buffer{buf: []byte(s)}
}
