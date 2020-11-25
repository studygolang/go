// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// errors 包实现了处理错误的函数。
//
// 函数 func New(text string) error 会创建仅包含文本信息的 error。
//
// 函数 Unwrap、Is 和 As 可以处理包装其他 error 的 error。
// 如果一个 error 具有该方法，则该 error 包含另一个 error。
//
//	Unwrap() error
//
// 如果 e.Unwrap() 返回了一个非空(nil)的错误 w，那么我们说 e 包装了 w。
//
// Unwrap 将解包被包装的 error。如果其参数拥有解包的方法，就调用它一次。否则将返回 nil。
//
// 可以通过调用 fmt.Errorf 并添加 %w 参数来简单创建一个包装了 error 的 error。
//
//	errors.Unwrap(fmt.Errorf("... %w ...", ..., err, ...))
//
// 该代码段返回 err 本身。
//
// Is 将按照顺序展开第一个参数，依次与第二个参数进行比较。他将报告是否找到匹配的 error。
// 他相对于简单的相等性检查来说，应当被优先使用(是更合适的方法)：
//
//	if errors.Is(err, fs.ErrExist)
//
// 比
//
//	if err == fs.ErrExist
//
// 更合适。
// 因为函数 Is 在 err 包含 fs.ErrExist 时也将返回 true。
//
// As 将按照顺序展开第一个参数，依次查找可分配给第二个参数（和第二个参数类型相符）的 error，第二个参数必须是一个指针。
// 如果成功找到了，则执行赋值并返回 true，否则返回 false。
//
// 这种形式：
//
//	var perr *fs.PathError
//	if errors.As(err, &perr) {
//		fmt.Println(perr.Path)
//	}
//
// 比
//
//	if perr, ok := err.(*fs.PathError); ok {
//		fmt.Println(perr.Path)
//	}
//
// 更合适。
// 因为当 err 包含 *fs.PathError 时，第一种形式也将返回成功。
package errors

// New 返回一个给定文本内容的 error。
// 每次调用 New 函数都将产生一个不同的 error，即使他们的文本内容是相同的。
func New(text string) error {
	return &errorString{text}
}

// errorString 是一个 error 的简单实现。
type errorString struct {
	s string
}

func (e *errorString) Error() string {
	return e.s
}
