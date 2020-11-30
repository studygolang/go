// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// log 包实现了简单的日志记录。 包中定义了 Logger 结构体类型，实现了一些格式化输出方法。
// 包中还提供了预定义“标准” Logger，可通过帮助函数 Print[f|ln]、Fatal[f|ln] 和 Panic[f|ln] 访问，比手动创建一个 Logger 更容易使用。
// 预定义 Logger 默认输出到标准错误，并且会打印每条日志信息的日期和时间。
// 每条日志信息输出在单独一行：如果要打印的信息没有以换行符结尾，Logger 会加一个换行符。
// Fatal 系列函数在写入日志信息后调用 os.Exit(1)。
// Panic 系列函数在写入日志信息后 panic。
package log

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// 这些选项定义了 Logger 生成每条日志条目的前缀文本内容。
// 将位组织在一起来控制打印内容。
// 除了 Lmsgprefix 这个选项，其他选项无法控制它们的显示顺序（这里列出的顺序）或它们显示格式（如评论中所述）。
// 当指定了 Llongfile 或者 Lshortfile 选项时，前缀后会跟一个冒号。
// 例如，Ldate | Ltime (or LstdFlags) 选项，
//	2009/01/23 01:23:23 message
// 而 Ldate | Ltime | Lmicroseconds | Llongfile 选项，
//	2009/01/23 01:23:23.123123 /a/b/c/d.go:23: message
const (
	Ldate         = 1 << iota     // 本地时区的日期: 2009/01/23
	Ltime                         // 本地时区的时间: 01:23:23
	Lmicroseconds                 // 微秒级: 01:23:23.123123。增强 Ltime
	Llongfile                     // 完整文件名和行号: /a/b/c/d.go:23
	Lshortfile                    // 文件名和行号: d.go:23. 会覆盖 Llongfile
	LUTC                          // 如果设置了 Ldate 或者 Ltime，使用 UTC 而不是本地时区
	Lmsgprefix                    // 将“前缀”从行的开头移至消息之前
	LstdFlags     = Ldate | Ltime // 标准 logger 的默认值
)

// Logger 表示一个活动的日志记录对象，它会生成一行行的输出到 io.Writer。
// 每次日志记录操作都会调用 Writer 的 Write 方法。
// Logger 可以多协程并行使用，它会保证对 Writer 的顺序访问。
type Logger struct {
	mu     sync.Mutex // 保证原子写; 保护以下字段
	prefix string     // 每行的前缀以识别 logger (请参考 Lmsgprefix)
	flag   int        // 属性
	out    io.Writer  // 输出目的地
	buf    []byte     // 为了缓冲文本再写入
}

// 新创建一个 Logger。参数 out 设置日志数据写入的目的地。
// 参数 prefix 会被添加到生成的每一条日志前面，如果使用 Lmsgprefix 选项，prefix 会被添加到日志头后面。
// 参数 flag 规定日志记录的属性。
func New(out io.Writer, prefix string, flag int) *Logger {
	return &Logger{out: out, prefix: prefix, flag: flag}
}

// SetOutput 设置 logger 输出的目的地
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
}

var std = New(os.Stderr, "", LstdFlags)

// Cheap integer to fixed-width decimal ASCII. Give a negative width to avoid zero-padding.
func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

// formatHeader writes log header to buf in following order:
//   * l.prefix (if it's not blank and Lmsgprefix is unset),
//   * date and/or time (if corresponding flags are provided),
//   * file and line number (if corresponding flags are provided),
//   * l.prefix (if it's not blank and Lmsgprefix is set).
func (l *Logger) formatHeader(buf *[]byte, t time.Time, file string, line int) {
	if l.flag&Lmsgprefix == 0 {
		*buf = append(*buf, l.prefix...)
	}
	if l.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		if l.flag&LUTC != 0 {
			t = t.UTC()
		}
		if l.flag&Ldate != 0 {
			year, month, day := t.Date()
			itoa(buf, year, 4)
			*buf = append(*buf, '/')
			itoa(buf, int(month), 2)
			*buf = append(*buf, '/')
			itoa(buf, day, 2)
			*buf = append(*buf, ' ')
		}
		if l.flag&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			itoa(buf, hour, 2)
			*buf = append(*buf, ':')
			itoa(buf, min, 2)
			*buf = append(*buf, ':')
			itoa(buf, sec, 2)
			if l.flag&Lmicroseconds != 0 {
				*buf = append(*buf, '.')
				itoa(buf, t.Nanosecond()/1e3, 6)
			}
			*buf = append(*buf, ' ')
		}
	}
	if l.flag&(Lshortfile|Llongfile) != 0 {
		if l.flag&Lshortfile != 0 {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
		}
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
		*buf = append(*buf, ": "...)
	}
	if l.flag&Lmsgprefix != 0 {
		*buf = append(*buf, l.prefix...)
	}
}

// Output 写入日志记录事件的输出。
// 字符串参数 s 包含要打印的文本，会被打印在 Logger 选项规定的特殊前缀后面。
// 如果 s 的末尾没有换行符，会在末尾添加换行符。
// 参数 Calldepth 用于恢复 PC，出于一般性提供，然后目前在所有预定义的路径上它的值都为2。
func (l *Logger) Output(calldepth int, s string) error {
	now := time.Now() // get this early.
	var file string
	var line int
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.flag&(Lshortfile|Llongfile) != 0 {
		// Release lock while getting caller info - it's expensive.
		l.mu.Unlock()
		var ok bool
		_, file, line, ok = runtime.Caller(calldepth)
		if !ok {
			file = "???"
			line = 0
		}
		l.mu.Lock()
	}
	l.buf = l.buf[:0]
	l.formatHeader(&l.buf, now, file, line)
	l.buf = append(l.buf, s...)
	if len(s) == 0 || s[len(s)-1] != '\n' {
		l.buf = append(l.buf, '\n')
	}
	_, err := l.out.Write(l.buf)
	return err
}

// Printf 调用 l.Output 输出到 logger。
// 以 fmt.Printf 的方式处理参数。
func (l *Logger) Printf(format string, v ...interface{}) {
	l.Output(2, fmt.Sprintf(format, v...))
}

// Print 调用 l.Output 输出到 logger。
// 以 fmt.Print 的方式处理参数。
func (l *Logger) Print(v ...interface{}) { l.Output(2, fmt.Sprint(v...)) }

// Println 调用 l.Output 输出到 logger。
// 以 fmt.Println 的方式处理参数。
func (l *Logger) Println(v ...interface{}) { l.Output(2, fmt.Sprintln(v...)) }

// Fatal 等价于 l.Print() 后调用 os.Exit(1)。
func (l *Logger) Fatal(v ...interface{}) {
	l.Output(2, fmt.Sprint(v...))
	os.Exit(1)
}

// Fatalf 等价于 l.Printf() 后调用 os.Exit(1)。
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.Output(2, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// Fatalln 等价于 l.Println() 后调用 os.Exit(1)。
func (l *Logger) Fatalln(v ...interface{}) {
	l.Output(2, fmt.Sprintln(v...))
	os.Exit(1)
}

// Panic 等价于 l.Print() 后调用 panic()。
func (l *Logger) Panic(v ...interface{}) {
	s := fmt.Sprint(v...)
	l.Output(2, s)
	panic(s)
}

// Panicf 等价于 l.Printf() 后调用 panic()。
func (l *Logger) Panicf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.Output(2, s)
	panic(s)
}

// Panicln 等价于 l.Println() 后调用 panic()。
func (l *Logger) Panicln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	l.Output(2, s)
	panic(s)
}

// Flags 返回 logger 的输出选项。
// 选项有 Ldate, Ltime 等。
func (l *Logger) Flags() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.flag
}

// SetFlags 设置 logger 的输出选项。
// 选项有 Ldate, Ltime 等。
func (l *Logger) SetFlags(flag int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.flag = flag
}

// Prefix 返回 logger 的输出前缀。
func (l *Logger) Prefix() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.prefix
}

// SetPrefix 设置 logger 的输出前缀。
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// Writer 返回 logger 的输出目的地。
func (l *Logger) Writer() io.Writer {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.out
}

// SetOutput 设置标准 logger 的输出目的地。
func SetOutput(w io.Writer) {
	std.mu.Lock()
	defer std.mu.Unlock()
	std.out = w
}

// Flags 返回标准 logger 的输出选项。
// 选项有 Ldate, Ltime 等。
func Flags() int {
	return std.Flags()
}

// SetFlags 设置标准 logger 的输出选项。
// 选项有 Ldate, Ltime 等。
func SetFlags(flag int) {
	std.SetFlags(flag)
}

// Prefix 返回标准 logger 的输出前缀。
func Prefix() string {
	return std.Prefix()
}

// SetPrefix 设置标准 logger 的输出前缀。
func SetPrefix(prefix string) {
	std.SetPrefix(prefix)
}

// Writer 返回标准 logger 的输出目的地。
func Writer() io.Writer {
	return std.Writer()
}

// These functions write to the standard logger.

// Print 调用 Output 输出到标准 logger。
// 以 fmt.Print 的方式处理参数。
func Print(v ...interface{}) {
	std.Output(2, fmt.Sprint(v...))
}

// Printf 调用 Output 输出到标准 logger。
// 以 fmt.Printf 的方式处理参数。
func Printf(format string, v ...interface{}) {
	std.Output(2, fmt.Sprintf(format, v...))
}

// Println 调用 Output 输出到标准 logger。
// 以 fmt.Println 的方式处理参数。
func Println(v ...interface{}) {
	std.Output(2, fmt.Sprintln(v...))
}

// Fatal 等价于 Print() 后调用 os.Exit(1)。
func Fatal(v ...interface{}) {
	std.Output(2, fmt.Sprint(v...))
	os.Exit(1)
}

// Fatalf 等价于 Printf() 后调用 os.Exit(1)。
func Fatalf(format string, v ...interface{}) {
	std.Output(2, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// Fatalln 等价于 Println() 后调用 os.Exit(1)。
func Fatalln(v ...interface{}) {
	std.Output(2, fmt.Sprintln(v...))
	os.Exit(1)
}

// Panic 等价于 Print() 后调用 panic()。
func Panic(v ...interface{}) {
	s := fmt.Sprint(v...)
	std.Output(2, s)
	panic(s)
}

// Panicf 等价于 Printf() 后调用 panic()。
func Panicf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	std.Output(2, s)
	panic(s)
}

// Panicln 等价于 Println() 后调用 panic()。
func Panicln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	std.Output(2, s)
	panic(s)
}

// Output 写入日志记录事件的输出。
// 字符串参数 s 包含要打印的文本，会被打印在 Logger 选项规定的特殊前缀后面。
// 如果 s 的末尾没有换行符，会在末尾添加换行符。
// Calldepth 是在设置了 Llongfile 或 Lshortfile 的情况下，处理文件名和行号时要跳过的帧数的计数；
// 为1时将打印 Output 调用者的详细信息。
func Output(calldepth int, s string) error {
	return std.Output(calldepth+1, s) // +1 for this frame.
}
