// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"internal/reflectlite"
)

// 如果参数 err 的 error 类型中包含 Unwrap 方法，则 Unwrap 函数将会返回对 err 调用 Unwrap 方法的结果，否则将返回 nil。
func Unwrap(err error) error {
	u, ok := err.(interface {
		Unwrap() error
	})
	if !ok {
		return nil
	}
	return u.Unwrap()
}

// Is 函数用于报告在参数 err 的错误链中是否包含任何一个与目标参数 target 相匹配的 error。
//
// 这个错误链由参数 err 本身和通过反复调用 Unwrap 获得的 error 序列组成。
//
// 当一个 error 和参数 target 相等或者它实现了 Is(error) bool 方法使得 Is(target) 返回 true 时，error 将被视为和参数 target 匹配成功。
//
// error 类型可能提供 Is 方法，因此可以将其视为等同于现有的 error。例如，如果 MyError 定义：
//
//	func (m MyError) Is(target error) bool { return target == os.ErrExist }
//
// 然后 Is(MyError{}, os.ErrExist) 返回 true。有关标准库中的建议，请参阅 syscall.Errno.Is。
func Is(err, target error) bool {
	if target == nil {
		return err == target
	}

	isComparable := reflectlite.TypeOf(target).Comparable()
	for {
		if isComparable && err == target {
			return true
		}
		if x, ok := err.(interface{ Is(error) bool }); ok && x.Is(target) {
			return true
		}
		// TODO: consider supporting target.Is(err). This would allow
		// user-definable predicates, but also may allow for coping with sloppy
		// APIs, thereby making it easier to get away with them.
		if err = Unwrap(err); err == nil {
			return false
		}
	}
}

// As 函数将查找在参数 err 的错误链中与参数 target 匹配的第一个 error，如果匹配到了，则将参数 target 设置为该 error 值并返回 true，否则返回 false。
//
// 这个错误链由参数 err 本身和通过反复调用 Unwrap 获得的 error 序列组成。
//
// 当一个 error 的具体值可以被赋值给参数 target 指向的值，或者 error 拥有 As(interface{}) bool 方法使得 As(target) 返回 true 时，error 将被视为和参数 target 匹配成功。
// 在后一种情况时，As 方法负责设置参数 target 的值。
//
// error 类型可能提供 As 方法，因此可以将其视为不同的 error 类型。
//
// 如果参数 target 不是指向一个实现 error 类型或者任何接口类型的非空指针，将会引发 panic。
func As(err error, target interface{}) bool {
	if target == nil {
		panic("errors: target cannot be nil")
	}
	val := reflectlite.ValueOf(target)
	typ := val.Type()
	if typ.Kind() != reflectlite.Ptr || val.IsNil() {
		panic("errors: target must be a non-nil pointer")
	}
	if e := typ.Elem(); e.Kind() != reflectlite.Interface && !e.Implements(errorType) {
		panic("errors: *target must be interface or implement error")
	}
	targetType := typ.Elem()
	for err != nil {
		if reflectlite.TypeOf(err).AssignableTo(targetType) {
			val.Elem().Set(reflectlite.ValueOf(err))
			return true
		}
		if x, ok := err.(interface{ As(interface{}) bool }); ok && x.As(target) {
			return true
		}
		err = Unwrap(err)
	}
	return false
}

var errorType = reflectlite.TypeOf((*error)(nil)).Elem()
