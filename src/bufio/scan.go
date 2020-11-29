// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bufio

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)


// Scanner 提供了一个很方便的接口来读取例如从以换行符作为分割的文件文本的数据。
// 连续的调用 Scan　方法会遍历一个文件中的 tokens，　跳过token之间的那些字符。
// token 是由类型为SplitFunc接口的分割函数定义的。　默认的分割函数是将输入根据行结束符分成多行。
// 分割函数在这里是为了将扫描的文件分成行，字节或者 UTF-8编码的字母，或者空格分割的单词等等。
// 客户端可以替换成自定义的分割函数。

// 对于 EOF, I/O error, token 相对于buffer太大 等报错都是无法恢复的停止。　当扫描停止了， reader 可能
// 距离上一个 token 很远了。　程序需要更多的错误处理或者更大的 token, 或者必须顺序扫描 reader ,建议用bufio.Reader。
//
type Scanner struct {
	r            io.Reader // The reader provided by the client.
	split        SplitFunc // The function to split the tokens.
	maxTokenSize int       // Maximum size of a token; modified by tests.
	token        []byte    // Last token returned by split.
	buf          []byte    // Buffer used as argument to split.
	start        int       // First non-processed byte in buf.
	end          int       // End of data in buf.
	err          error     // Sticky error.
	empties      int       // Count of successive empty tokens.
	scanCalled   bool      // Scan has been called; buffer is in use.
	done         bool      // Scan has finished.
}

// SplitFunc　是用来切分词(tokenize)的分割函数的函数签名。
// 参数应该是一些尚未处理过的数据和 flag, atEOF, 用来的判断是否 reader 还有更多的数据。
// 这个的返回值是前进(或者跳过)的字节数,　返回　token　给用户,　如果有 error　则返回 error。
//
// 扫描会停止，如果函数返回了 error, 在这种情况下，部分输入可能会被丢弃掉。
//
// 正常情况下， Scanner 会继续向前扫描 input, 如果　token 不为 nil, 那么将返回给用户。
// 如果　token 是 nil, 那么　Scanner 将会读取更多的数据并继续扫描，　如果数据扫描完，也就是　atEOF 为 true　时，
// Scanner　会返回。如果数据并没有一个 token 也没有，　例如 在扫描多行时没有换行符，　 SplitFunc 可以返回 (0, nil, nil) 来
// 通知　Scanner 去读取更多的数据到 slice　中并重试。
//
// SplitFunc 除非是 atEOF 为 true, 否则从不返回空的　slice。　当然，　data　也可能非空，并且总是持有未处理的数据。
type SplitFunc func(data []byte, atEOF bool) (advance int, token []byte, err error)


// Scanner 中会返回的 Errors。
var (
	ErrTooLong         = errors.New("bufio.Scanner: token too long")
	ErrNegativeAdvance = errors.New("bufio.Scanner: SplitFunc returns negative advance count")
	ErrAdvanceTooFar   = errors.New("bufio.Scanner: SplitFunc returns advance count beyond input")
	ErrBadReadCount    = errors.New("bufio.Scanner: Read returned impossible count")
)

const (
	// MaxScanTokenSize 是　buffer　中一个 token 的最大长度, 除非用户自己指定了一个实现了 Scanner.Buffer的　buffer。
	// 实际的最大 token 长度　可能会小于这个值，　原因是可能还要存放其他的一些数据，比如 换行符等等。
	MaxScanTokenSize = 64 * 1024

	startBufSize = 4096 // Size of initial allocation for buffer.
)

// NewScanner　new 了一个　Scanner对象来从　r　中读取数据。
// 默认的分割函数是 ScanLines。
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{
		r:            r,
		split:        ScanLines,
		maxTokenSize: MaxScanTokenSize,
	}
}

// Err 返回　Scanner遇到的　非EOF(non-EOF)　的错误。
func (s *Scanner) Err() error {
	if s.err == io.EOF {
		return nil
	}
	return s.err
}

// Bytes 返回最近由调用Scan生成的 token。
// 底层数组的数据可能会被下一次 Scan 的调用覆盖掉。期间不做内存分配。
func (s *Scanner) Bytes() []byte {
	return s.token
}

// Text 返回最近由调用Scan生成的 token作为一个　string　返回。
func (s *Scanner) Text() string {
	return string(s.token)
}

// ErrFinalToken 是一个特殊的错误值哨兵。　作用是告诉 Scanner 这是最后一个 token, 并且扫描接下来可以暂停了。
// 当　Scan　收到　ErrFinalToken，扫描将结束，并且 err == nil。
// 当需要早点结束处理或者需要传送一个最后的 空 token 时很有用。 当然也可以自定义一个 error 来做同样的操作，放在这里比较简洁。
// 可以参照 emptyFinalToken 的例子来了解这个值的用法。
var ErrFinalToken = errors.New("final token")

// Scan 移动 Scanner 到下一个 可以处理的token。 当扫描结束时返回 false ， Err 方法将会
// 返回扫描中的错误， io.EOF 是个例外， 此时会返回 nil。
func (s *Scanner) Scan() bool {
	if s.done {
		return false
	}
	s.scanCalled = true
	// Loop until we have a token.
	for {
		// See if we can get a token with what we already have.
		// If we've run out of data but have an error, give the split function
		// a chance to recover any remaining, possibly empty token.
		if s.end > s.start || s.err != nil {
			advance, token, err := s.split(s.buf[s.start:s.end], s.err != nil)
			if err != nil {
				if err == ErrFinalToken {
					s.token = token
					s.done = true
					return true
				}
				s.setErr(err)
				return false
			}
			if !s.advance(advance) {
				return false
			}
			s.token = token
			if token != nil {
				if s.err == nil || advance > 0 {
					s.empties = 0
				} else {
					// Returning tokens not advancing input at EOF.
					s.empties++
					if s.empties > maxConsecutiveEmptyReads {
						panic("bufio.Scan: too many empty tokens without progressing")
					}
				}
				return true
			}
		}
		// We cannot generate a token with what we are holding.
		// If we've already hit EOF or an I/O error, we are done.
		if s.err != nil {
			// Shut it down.
			s.start = 0
			s.end = 0
			return false
		}
		// Must read more data.
		// First, shift data to beginning of buffer if there's lots of empty space
		// or space is needed.
		if s.start > 0 && (s.end == len(s.buf) || s.start > len(s.buf)/2) {
			copy(s.buf, s.buf[s.start:s.end])
			s.end -= s.start
			s.start = 0
		}
		// Is the buffer full? If so, resize.
		if s.end == len(s.buf) {
			// Guarantee no overflow in the multiplication below.
			const maxInt = int(^uint(0) >> 1)
			if len(s.buf) >= s.maxTokenSize || len(s.buf) > maxInt/2 {
				s.setErr(ErrTooLong)
				return false
			}
			newSize := len(s.buf) * 2
			if newSize == 0 {
				newSize = startBufSize
			}
			if newSize > s.maxTokenSize {
				newSize = s.maxTokenSize
			}
			newBuf := make([]byte, newSize)
			copy(newBuf, s.buf[s.start:s.end])
			s.buf = newBuf
			s.end -= s.start
			s.start = 0
		}
		// Finally we can read some input. Make sure we don't get stuck with
		// a misbehaving Reader. Officially we don't need to do this, but let's
		// be extra careful: Scanner is for safe, simple jobs.
		for loop := 0; ; {
			n, err := s.r.Read(s.buf[s.end:len(s.buf)])
			if n < 0 || len(s.buf)-s.end < n {
				s.setErr(ErrBadReadCount)
				break
			}
			s.end += n
			if err != nil {
				s.setErr(err)
				break
			}
			if n > 0 {
				s.empties = 0
				break
			}
			loop++
			if loop > maxConsecutiveEmptyReads {
				s.setErr(io.ErrNoProgress)
				break
			}
		}
	}
}

// advance consumes n bytes of the buffer. It reports whether the advance was legal.
func (s *Scanner) advance(n int) bool {
	if n < 0 {
		s.setErr(ErrNegativeAdvance)
		return false
	}
	if n > s.end-s.start {
		s.setErr(ErrAdvanceTooFar)
		return false
	}
	s.start += n
	return true
}

// setErr records the first error encountered.
func (s *Scanner) setErr(err error) {
	if s.err == nil || s.err == io.EOF {
		s.err = err
	}
}

// Buffer 设置在扫描过程中需要用到的 buffer 和 buffer 能申请的的最大长度。
// 最大值 为 max 和 cap(buf) 中的最大值。
//
// 默认情况下， Scan 将使用内部的 buffer 并且设置最大长度到 MaxScanTokenSize。
//
// 如果扫描已经开始了那么调用 Buffer 将会 panic。
//
func (s *Scanner) Buffer(buf []byte, max int) {
	if s.scanCalled {
		panic("Buffer called after Scan")
	}
	s.buf = buf[0:cap(buf)]
	s.maxTokenSize = max
}

// Split 为 Scanner 设置分割函数。
// 默认的分割函数为 ScanLines。
//
// 如果扫描已经开始了那么调用 Split 将会 panic。
func (s *Scanner) Split(split SplitFunc) {
	if s.scanCalled {
		panic("Split called after Scan")
	}
	s.split = split
}

// 分割函数

// ScanBytes 每次只返回一个字节当 token。
func ScanBytes(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	return 1, data[0:1], nil
}

var errorRune = []byte(string(utf8.RuneError))

// ScanRunes 将每一个 UTF-8-encoded 字符当做 token。 这个字符序列类似于对一个 string 做 for 循环，
// 也就是说错误的UTF-8编码将会翻译为 U+FFFD = "\xef\xbf\xbd" 。
// 因为 Scan 接口， 这也使得客户端能区分编码失败。
func ScanRunes(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Fast path 1: ASCII.
	if data[0] < utf8.RuneSelf {
		return 1, data[0:1], nil
	}

	// Fast path 2: Correct UTF-8 decode without error.
	_, width := utf8.DecodeRune(data)
	if width > 1 {
		// It's a valid encoding. Width cannot be one for a correctly encoded
		// non-ASCII rune.
		return width, data[0:width], nil
	}

	// We know it's an error: we have width==1 and implicitly r==utf8.RuneError.
	// Is the error because there wasn't a full rune to be decoded?
	// FullRune distinguishes correctly between erroneous and incomplete encodings.
	if !atEOF && !utf8.FullRune(data) {
		// Incomplete; get more bytes.
		return 0, nil, nil
	}

	// We have a real UTF-8 encoding error. Return a properly encoded error rune
	// but advance only one byte. This matches the behavior of a range loop over
	// an incorrectly encoded string.
	return 1, errorRune, nil
}

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

// ScanLines is a split function for a Scanner that returns each line of
// text, stripped of any trailing end-of-line marker. The returned line may
// be empty. The end-of-line marker is one optional carriage return followed
// by one mandatory newline. In regular expression notation, it is `\r?\n`.
// The last non-empty line of input will be returned even if it has no
// newline.

// ScanLines 返回每一行数据作为 token，并且去掉了行尾标记。 返回的一行数据可能为空。
// 行尾标记是一个可选的字符在每一行的最后。通常是 `\r?\n`。
// 最后一个非空行将会正常返回，即使没有换行符。
func ScanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, dropCR(data[0:i]), nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	// Request more data.
	return 0, nil, nil
}

// isSpace reports whether the character is a Unicode white space character.
// We avoid dependency on the unicode package, but check validity of the implementation
// in the tests.
func isSpace(r rune) bool {
	if r <= '\u00FF' {
		// Obvious ASCII ones: \t through \r plus space. Plus two Latin-1 oddballs.
		switch r {
		case ' ', '\t', '\n', '\v', '\f', '\r':
			return true
		case '\u0085', '\u00A0':
			return true
		}
		return false
	}
	// High-valued ones.
	if '\u2000' <= r && r <= '\u200a' {
		return true
	}
	switch r {
	case '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000':
		return true
	}
	return false
}

// ScanWords 是将输入按空格分割为 token。并且去掉了两边的空格。
// 这个永远不会返回空字符串。 space 的定义为 unicode.IsSpace。
func ScanWords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Skip leading spaces.
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if !isSpace(r) {
			break
		}
	}
	// Scan until space, marking end of word.
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		if isSpace(r) {
			return i + width, data[start:i], nil
		}
	}
	// If we're at EOF, we have a final, non-empty, non-terminated word. Return it.
	if atEOF && len(data) > start {
		return len(data), data[start:], nil
	}
	// Request more data.
	return start, nil, nil
}
