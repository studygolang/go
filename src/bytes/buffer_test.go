// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytes_test

import (
	. "bytes"
	"io"
	"math/rand"
	"testing"
	"unicode/utf8"
)

const N = 10000       // 为更大(和更慢)的测试使这个值大一些
var testString string // 用于写测试的测试数据
var testBytes []byte  // 测试数据;与 testString 相同，只是作为一个切片。

type negativeReader struct{}

func (r *negativeReader) Read([]byte) (int, error) { return -1, nil }

func init() {
	testBytes = make([]byte, N)
	for i := 0; i < N; i++ {
		testBytes[i] = 'a' + byte(i%26)
	}
	testString = string(testBytes)
}

// 验证 buf 的内容与字符串 s 匹配。
func check(t *testing.T, testname string, buf *Buffer, s string) {
	bytes := buf.Bytes()
	str := buf.String()
	if buf.Len() != len(bytes) {
		t.Errorf("%s: buf.Len() == %d, len(buf.Bytes()) == %d", testname, buf.Len(), len(bytes))
	}

	if buf.Len() != len(str) {
		t.Errorf("%s: buf.Len() == %d, len(buf.String()) == %d", testname, buf.Len(), len(str))
	}

	if buf.Len() != len(s) {
		t.Errorf("%s: buf.Len() == %d, len(s) == %d", testname, buf.Len(), len(s))
	}

	if string(bytes) != s {
		t.Errorf("%s: string(buf.Bytes()) == %q, s == %q", testname, string(bytes), s)
	}
}

// 通过字符串 fub 的 n 次写入填充 buf。
// buf 的初始内容对应于字符串 s；
// 结果是以字符串形式返回的 buf 的最终内容。
func fillString(t *testing.T, testname string, buf *Buffer, s string, n int, fus string) string {
	check(t, testname+" (fill 1)", buf, s)
	for ; n > 0; n-- {
		m, err := buf.WriteString(fus)
		if m != len(fus) {
			t.Errorf(testname+" (fill 2): m == %d, expected %d", m, len(fus))
		}
		if err != nil {
			t.Errorf(testname+" (fill 3): err should always be nil, found err == %s", err)
		}
		s += fus
		check(t, testname+" (fill 4)", buf, s)
	}
	return s
}

// 通过字节切片 fub 的 n 次写入填充 buf。
// buf 的初始内容对应于字符串 s；
// 结果是以字符串形式返回的 buf 的最终内容。
func fillBytes(t *testing.T, testname string, buf *Buffer, s string, n int, fub []byte) string {
	check(t, testname+" (fill 1)", buf, s)
	for ; n > 0; n-- {
		m, err := buf.Write(fub)
		if m != len(fub) {
			t.Errorf(testname+" (fill 2): m == %d, expected %d", m, len(fub))
		}
		if err != nil {
			t.Errorf(testname+" (fill 3): err should always be nil, found err == %s", err)
		}
		s += string(fub)
		check(t, testname+" (fill 4)", buf, s)
	}
	return s
}

func TestNewBuffer(t *testing.T) {
	buf := NewBuffer(testBytes)
	check(t, "NewBuffer", buf, testString)
}

func TestNewBufferString(t *testing.T) {
	buf := NewBufferString(testString)
	check(t, "NewBufferString", buf, testString)
}

// 空 buf 通过反复读变成 fub。
// buf 的初始内容对应于字符串 s。
func empty(t *testing.T, testname string, buf *Buffer, s string, fub []byte) {
	check(t, testname+" (empty 1)", buf, s)

	for {
		n, err := buf.Read(fub)
		if n == 0 {
			break
		}
		if err != nil {
			t.Errorf(testname+" (empty 2): err should always be nil, found err == %s", err)
		}
		s = s[n:]
		check(t, testname+" (empty 3)", buf, s)
	}

	check(t, testname+" (empty 4)", buf, "")
}

func TestBasicOperations(t *testing.T) {
	var buf Buffer

	for i := 0; i < 5; i++ {
		check(t, "TestBasicOperations (1)", &buf, "")

		buf.Reset()
		check(t, "TestBasicOperations (2)", &buf, "")

		buf.Truncate(0)
		check(t, "TestBasicOperations (3)", &buf, "")

		n, err := buf.Write(testBytes[0:1])
		if want := 1; err != nil || n != want {
			t.Errorf("Write: got (%d, %v), want (%d, %v)", n, err, want, nil)
		}
		check(t, "TestBasicOperations (4)", &buf, "a")

		buf.WriteByte(testString[1])
		check(t, "TestBasicOperations (5)", &buf, "ab")

		n, err = buf.Write(testBytes[2:26])
		if want := 24; err != nil || n != want {
			t.Errorf("Write: got (%d, %v), want (%d, %v)", n, err, want, nil)
		}
		check(t, "TestBasicOperations (6)", &buf, testString[0:26])

		buf.Truncate(26)
		check(t, "TestBasicOperations (7)", &buf, testString[0:26])

		buf.Truncate(20)
		check(t, "TestBasicOperations (8)", &buf, testString[0:20])

		empty(t, "TestBasicOperations (9)", &buf, testString[0:20], make([]byte, 5))
		empty(t, "TestBasicOperations (10)", &buf, "", make([]byte, 100))

		buf.WriteByte(testString[1])
		c, err := buf.ReadByte()
		if want := testString[1]; err != nil || c != want {
			t.Errorf("ReadByte: got (%q, %v), want (%q, %v)", c, err, want, nil)
		}
		c, err = buf.ReadByte()
		if err != io.EOF {
			t.Errorf("ReadByte: got (%q, %v), want (%q, %v)", c, err, byte(0), io.EOF)
		}
	}
}

func TestLargeStringWrites(t *testing.T) {
	var buf Buffer
	limit := 30
	if testing.Short() {
		limit = 9
	}
	for i := 3; i < limit; i += 3 {
		s := fillString(t, "TestLargeWrites (1)", &buf, "", 5, testString)
		empty(t, "TestLargeStringWrites (2)", &buf, s, make([]byte, len(testString)/i))
	}
	check(t, "TestLargeStringWrites (3)", &buf, "")
}

func TestLargeByteWrites(t *testing.T) {
	var buf Buffer
	limit := 30
	if testing.Short() {
		limit = 9
	}
	for i := 3; i < limit; i += 3 {
		s := fillBytes(t, "TestLargeWrites (1)", &buf, "", 5, testBytes)
		empty(t, "TestLargeByteWrites (2)", &buf, s, make([]byte, len(testString)/i))
	}
	check(t, "TestLargeByteWrites (3)", &buf, "")
}

func TestLargeStringReads(t *testing.T) {
	var buf Buffer
	for i := 3; i < 30; i += 3 {
		s := fillString(t, "TestLargeReads (1)", &buf, "", 5, testString[0:len(testString)/i])
		empty(t, "TestLargeReads (2)", &buf, s, make([]byte, len(testString)))
	}
	check(t, "TestLargeStringReads (3)", &buf, "")
}

func TestLargeByteReads(t *testing.T) {
	var buf Buffer
	for i := 3; i < 30; i += 3 {
		s := fillBytes(t, "TestLargeReads (1)", &buf, "", 5, testBytes[0:len(testBytes)/i])
		empty(t, "TestLargeReads (2)", &buf, s, make([]byte, len(testString)))
	}
	check(t, "TestLargeByteReads (3)", &buf, "")
}

func TestMixedReadsAndWrites(t *testing.T) {
	var buf Buffer
	s := ""
	for i := 0; i < 50; i++ {
		wlen := rand.Intn(len(testString))
		if i%2 == 0 {
			s = fillString(t, "TestMixedReadsAndWrites (1)", &buf, s, 1, testString[0:wlen])
		} else {
			s = fillBytes(t, "TestMixedReadsAndWrites (1)", &buf, s, 1, testBytes[0:wlen])
		}

		rlen := rand.Intn(len(testString))
		fub := make([]byte, rlen)
		n, _ := buf.Read(fub)
		s = s[n:]
	}
	empty(t, "TestMixedReadsAndWrites (2)", &buf, s, make([]byte, buf.Len()))
}

func TestCapWithPreallocatedSlice(t *testing.T) {
	buf := NewBuffer(make([]byte, 10))
	n := buf.Cap()
	if n != 10 {
		t.Errorf("expected 10, got %d", n)
	}
}

func TestCapWithSliceAndWrittenData(t *testing.T) {
	buf := NewBuffer(make([]byte, 0, 10))
	buf.Write([]byte("test"))
	n := buf.Cap()
	if n != 10 {
		t.Errorf("expected 10, got %d", n)
	}
}

func TestNil(t *testing.T) {
	var b *Buffer
	if b.String() != "<nil>" {
		t.Errorf("expected <nil>; got %q", b.String())
	}
}

func TestReadFrom(t *testing.T) {
	var buf Buffer
	for i := 3; i < 30; i += 3 {
		s := fillBytes(t, "TestReadFrom (1)", &buf, "", 5, testBytes[0:len(testBytes)/i])
		var b Buffer
		b.ReadFrom(&buf)
		empty(t, "TestReadFrom (2)", &b, s, make([]byte, len(testString)))
	}
}

type panicReader struct{ panic bool }

func (r panicReader) Read(p []byte) (int, error) {
	if r.panic {
		panic(nil)
	}
	return 0, io.EOF
}

// Make sure that an empty Buffer remains empty when
// it is "grown" before a Read that panics
func TestReadFromPanicReader(t *testing.T) {

	// 首先验证非 panic 行为
	var buf Buffer
	i, err := buf.ReadFrom(panicReader{})
	if err != nil {
		t.Fatal(err)
	}
	if i != 0 {
		t.Fatalf("unexpected return from bytes.ReadFrom (1): got: %d, want %d", i, 0)
	}
	check(t, "TestReadFromPanicReader (1)", &buf, "")

	// 确认当 Reader 发生 panic 时，空缓冲区仍然是空的
	var buf2 Buffer
	defer func() {
		recover()
		check(t, "TestReadFromPanicReader (2)", &buf2, "")
	}()
	buf2.ReadFrom(panicReader{panic: true})
}

func TestReadFromNegativeReader(t *testing.T) {
	var b Buffer
	defer func() {
		switch err := recover().(type) {
		case nil:
			t.Fatal("bytes.Buffer.ReadFrom didn't panic")
		case error:
			// 这是 errNegativeRead 的错误字符串
			wantError := "bytes.Buffer: reader returned negative count from Read"
			if err.Error() != wantError {
				t.Fatalf("recovered panic: got %v, want %v", err.Error(), wantError)
			}
		default:
			t.Fatalf("unexpected panic value: %#v", err)
		}
	}()

	b.ReadFrom(new(negativeReader))
}

func TestWriteTo(t *testing.T) {
	var buf Buffer
	for i := 3; i < 30; i += 3 {
		s := fillBytes(t, "TestWriteTo (1)", &buf, "", 5, testBytes[0:len(testBytes)/i])
		var b Buffer
		buf.WriteTo(&b)
		empty(t, "TestWriteTo (2)", &b, s, make([]byte, len(testString)))
	}
}

func TestRuneIO(t *testing.T) {
	const NRune = 1000
	// 在写入数据时构建一个测试切片
	b := make([]byte, utf8.UTFMax*NRune)
	var buf Buffer
	n := 0
	for r := rune(0); r < NRune; r++ {
		size := utf8.EncodeRune(b[n:], r)
		nbytes, err := buf.WriteRune(r)
		if err != nil {
			t.Fatalf("WriteRune(%U) error: %s", r, err)
		}
		if nbytes != size {
			t.Fatalf("WriteRune(%U) expected %d, got %d", r, size, nbytes)
		}
		n += size
	}
	b = b[0:n]

	// 检查结果字节
	if !Equal(buf.Bytes(), b) {
		t.Fatalf("incorrect result from WriteRune: %q not %q", buf.Bytes(), b)
	}

	p := make([]byte, utf8.UTFMax)
	// 用 ReadRune 把它读回来
	for r := rune(0); r < NRune; r++ {
		size := utf8.EncodeRune(p, r)
		nr, nbytes, err := buf.ReadRune()
		if nr != r || nbytes != size || err != nil {
			t.Fatalf("ReadRune(%U) got %U,%d not %U,%d (err=%s)", r, nr, nbytes, r, size, err)
		}
	}

	// 检查 UnreadRune 是否有效
	buf.Reset()

	// 在 EOF 检查
	if err := buf.UnreadRune(); err == nil {
		t.Fatal("UnreadRune at EOF: got no error")
	}
	if _, _, err := buf.ReadRune(); err == nil {
		t.Fatal("ReadRune at EOF: got no error")
	}
	if err := buf.UnreadRune(); err == nil {
		t.Fatal("UnreadRune after ReadRune at EOF: got no error")
	}

	// 不在 EOF 检查
	buf.Write(b)
	for r := rune(0); r < NRune; r++ {
		r1, size, _ := buf.ReadRune()
		if err := buf.UnreadRune(); err != nil {
			t.Fatalf("UnreadRune(%U) got error %q", r, err)
		}
		r2, nbytes, err := buf.ReadRune()
		if r1 != r2 || r1 != r || nbytes != size || err != nil {
			t.Fatalf("ReadRune(%U) after UnreadRune got %U,%d not %U,%d (err=%s)", r, r2, nbytes, r, size, err)
		}
	}
}

func TestNext(t *testing.T) {
	b := []byte{0, 1, 2, 3, 4}
	tmp := make([]byte, 5)
	for i := 0; i <= 5; i++ {
		for j := i; j <= 5; j++ {
			for k := 0; k <= 6; k++ {
				// 0 <= i <= j <= 5; 0 <= k <= 6
				// 如果我们在偏移量 i 处以长度为 j 的缓冲区开始检查，
				// 然后请求 Next(k)，我们就得到了正确的字节。
				buf := NewBuffer(b[0:j])
				n, _ := buf.Read(tmp[0:i])
				if n != i {
					t.Fatalf("Read %d returned %d", i, n)
				}
				bb := buf.Next(k)
				want := k
				if want > j-i {
					want = j - i
				}
				if len(bb) != want {
					t.Fatalf("in %d,%d: len(Next(%d)) == %d", i, j, k, len(bb))
				}
				for l, v := range bb {
					if v != byte(l+i) {
						t.Fatalf("in %d,%d: Next(%d)[%d] = %d, want %d", i, j, k, l, v, l+i)
					}
				}
			}
		}
	}
}

var readBytesTests = []struct {
	buffer   string
	delim    byte
	expected []string
	err      error
}{
	{"", 0, []string{""}, io.EOF},
	{"a\x00", 0, []string{"a\x00"}, nil},
	{"abbbaaaba", 'b', []string{"ab", "b", "b", "aaab"}, nil},
	{"hello\x01world", 1, []string{"hello\x01"}, nil},
	{"foo\nbar", 0, []string{"foo\nbar"}, io.EOF},
	{"alpha\nbeta\ngamma\n", '\n', []string{"alpha\n", "beta\n", "gamma\n"}, nil},
	{"alpha\nbeta\ngamma", '\n', []string{"alpha\n", "beta\n", "gamma"}, io.EOF},
}

func TestReadBytes(t *testing.T) {
	for _, test := range readBytesTests {
		buf := NewBufferString(test.buffer)
		var err error
		for _, expected := range test.expected {
			var bytes []byte
			bytes, err = buf.ReadBytes(test.delim)
			if string(bytes) != expected {
				t.Errorf("expected %q, got %q", expected, bytes)
			}
			if err != nil {
				break
			}
		}
		if err != test.err {
			t.Errorf("expected error %v, got %v", test.err, err)
		}
	}
}

func TestReadString(t *testing.T) {
	for _, test := range readBytesTests {
		buf := NewBufferString(test.buffer)
		var err error
		for _, expected := range test.expected {
			var s string
			s, err = buf.ReadString(test.delim)
			if s != expected {
				t.Errorf("expected %q, got %q", expected, s)
			}
			if err != nil {
				break
			}
		}
		if err != test.err {
			t.Errorf("expected error %v, got %v", test.err, err)
		}
	}
}

func BenchmarkReadString(b *testing.B) {
	const n = 32 << 10

	data := make([]byte, n)
	data[n-1] = 'x'
	b.SetBytes(int64(n))
	for i := 0; i < b.N; i++ {
		buf := NewBuffer(data)
		_, err := buf.ReadString('x')
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestGrow(t *testing.T) {
	x := []byte{'x'}
	y := []byte{'y'}
	tmp := make([]byte, 72)
	for _, growLen := range []int{0, 100, 1000, 10000, 100000} {
		for _, startLen := range []int{0, 100, 1000, 10000, 100000} {
			xBytes := Repeat(x, startLen)

			buf := NewBuffer(xBytes)
			// 如果发生读，会影响 buf.off，这是很好的测试。
			readBytes, _ := buf.Read(tmp)
			yBytes := Repeat(y, growLen)
			allocs := testing.AllocsPerRun(100, func() {
				buf.Grow(growLen)
				buf.Write(yBytes)
			})
			// 只要是单线程，就不会在写的时候发生分配。
			if allocs != 0 {
				t.Errorf("allocation occurred during write")
			}
			// 检查缓冲区有正确的数据。
			if !Equal(buf.Bytes()[0:startLen-readBytes], xBytes[readBytes:]) {
				t.Errorf("bad initial data at %d %d", startLen, growLen)
			}
			if !Equal(buf.Bytes()[startLen-readBytes:startLen-readBytes+growLen], yBytes) {
				t.Errorf("bad written data at %d %d", startLen, growLen)
			}
		}
	}
}

func TestGrowOverflow(t *testing.T) {
	defer func() {
		if err := recover(); err != ErrTooLarge {
			t.Errorf("after too-large Grow, recover() = %v; want %v", err, ErrTooLarge)
		}
	}()

	buf := NewBuffer(make([]byte, 1))
	const maxInt = int(^uint(0) >> 1)
	buf.Grow(maxInt)
}

// Was a bug: 用来在 EOF 给 EOF 读空切片。
func TestReadEmptyAtEOF(t *testing.T) {
	b := new(Buffer)
	slice := make([]byte, 0)
	n, err := b.Read(slice)
	if err != nil {
		t.Errorf("read error: %v", err)
	}
	if n != 0 {
		t.Errorf("wrong count; got %d want 0", n)
	}
}

func TestUnreadByte(t *testing.T) {
	b := new(Buffer)

	// 在 EOF 检查
	if err := b.UnreadByte(); err == nil {
		t.Fatal("UnreadByte at EOF: got no error")
	}
	if _, err := b.ReadByte(); err == nil {
		t.Fatal("ReadByte at EOF: got no error")
	}
	if err := b.UnreadByte(); err == nil {
		t.Fatal("UnreadByte after ReadByte at EOF: got no error")
	}

	// 不在 EOF 检查
	b.WriteString("abcdefghijklmnopqrstuvwxyz")

	// 在失败的读取之后
	if n, err := b.Read(nil); n != 0 || err != nil {
		t.Fatalf("Read(nil) = %d,%v; want 0,nil", n, err)
	}
	if err := b.UnreadByte(); err == nil {
		t.Fatal("UnreadByte after Read(nil): got no error")
	}

	// 在成功的读取之后
	if _, err := b.ReadBytes('m'); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if err := b.UnreadByte(); err != nil {
		t.Fatalf("UnreadByte: %v", err)
	}
	c, err := b.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if c != 'm' {
		t.Errorf("ReadByte = %q; want %q", c, 'm')
	}
}

// 我们偶尔进行压缩测试。 Issue 5154.
func TestBufferGrowth(t *testing.T) {
	var b Buffer
	buf := make([]byte, 1024)
	b.Write(buf[0:1])
	var cap0 int
	for i := 0; i < 5<<10; i++ {
		b.Write(buf)
		b.Read(buf)
		if i == 0 {
			cap0 = b.Cap()
		}
	}
	cap1 := b.Cap()
	// (*Buffer).grow 允许在滑动前有 2x 的容量斜率，
	// 因此将我们的错误阈值设置为 3x。
	if cap1 > cap0*3 {
		t.Errorf("buffer cap = %d; too big (grew from %d)", cap1, cap0)
	}
}

func BenchmarkWriteByte(b *testing.B) {
	const n = 4 << 10
	b.SetBytes(n)
	buf := NewBuffer(make([]byte, n))
	for i := 0; i < b.N; i++ {
		buf.Reset()
		for i := 0; i < n; i++ {
			buf.WriteByte('x')
		}
	}
}

func BenchmarkWriteRune(b *testing.B) {
	const n = 4 << 10
	const r = '☺'
	b.SetBytes(int64(n * utf8.RuneLen(r)))
	buf := NewBuffer(make([]byte, n*utf8.UTFMax))
	for i := 0; i < b.N; i++ {
		buf.Reset()
		for i := 0; i < n; i++ {
			buf.WriteRune(r)
		}
	}
}

// 来自 Issue 5154.
func BenchmarkBufferNotEmptyWriteRead(b *testing.B) {
	buf := make([]byte, 1024)
	for i := 0; i < b.N; i++ {
		var b Buffer
		b.Write(buf[0:1])
		for i := 0; i < 5<<10; i++ {
			b.Write(buf)
			b.Read(buf)
		}
	}
}

// 检查一下我们没有太过频繁地压缩。 来自 Issue 5154.
func BenchmarkBufferFullSmallReads(b *testing.B) {
	buf := make([]byte, 1024)
	for i := 0; i < b.N; i++ {
		var b Buffer
		b.Write(buf)
		for b.Len()+20 < b.Cap() {
			b.Write(buf[:10])
		}
		for i := 0; i < 5<<10; i++ {
			b.Read(buf[:1])
			b.Write(buf[:1])
		}
	}
}
