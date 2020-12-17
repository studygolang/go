// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Atomic 包提供了底层的内存原子操作，对并发的实现非常必要.
//
// 这些方法想要正确的应用需要格外的注意.
// 除底层的应用程序外，最好使用channel或sync包来实现并发控制
//「下面是go经典语录，建议结合实际去理解」
// Share memory by communicating;
// don't communicate by sharing memory.
//
// swap操作通过SwapT函数实现is the atomic
// 在原子上等效于:
//
//	old = *addr
//	*addr = new
//	return old
//
// compare-and-swap操作通过CompareAndSwapT函数实现
// 在原子上等效于:
//
//	if *addr == old {
//		*addr = new
//		return true
//	}
//	return false
//
// add操作通过AddT函数实现
// 在原子上等效于:
//
//	*addr += delta
//	return *addr
//
// load 和 store 操作, 通过 LoadT and StoreT 函数实现
// 在原子上等效于 "return *addr" 和 "*addr = val".
//
package atomic

import (
	"unsafe"
)

// BUG(rsc): 在 x86 32位下,  64位的函数操作只能兼容到Pentium(奔腾) MMX架构.
//
// 在非Linux ARM下, 64位的函数操作只能兼容到 ARMv6k 内核支持的指令.
//
// 在 ARM, x86-32, 和 32-bit MIPS 上,
// 调用方在使用64位函数的操作，需要自行做好64位对齐
// 可以依赖变量 struct、array、slice中的第一个字进行64位对齐

// SwapInt32 将 new 的原子的存储到 *addr 中，并返回 *addr 之前的值.
func SwapInt32(addr *int32, new int32) (old int32)

// SwapInt64 将 new 的原子的存储到 *addr 中，并返回 *addr 之前的值.
func SwapInt64(addr *int64, new int64) (old int64)

// SwapUint32 将 new 的原子的存储到 *addr 中，并返回 *addr 之前的值.
func SwapUint32(addr *uint32, new uint32) (old uint32)

// SwapUint64 将 new 的原子的存储到 *addr 中，并返回 *addr 之前的值.
func SwapUint64(addr *uint64, new uint64) (old uint64)

// SwapUintptr 将 new 的原子的存储到 *addr 中，并返回 *addr 之前的值.
func SwapUintptr(addr *uintptr, new uintptr) (old uintptr)

// SwapPointer 将 new 的原子的存储到 *addr 中，并返回 *addr 之前的值.
func SwapPointer(addr *unsafe.Pointer, new unsafe.Pointer) (old unsafe.Pointer)

// CompareAndSwapInt32 适用于 Int32 的对比并交换；先对比 new 是否与 old 相同，如果相同就交换并返回 true，如果不同直接返回 false.
func CompareAndSwapInt32(addr *int32, old, new int32) (swapped bool)

// CompareAndSwapInt64 适用于 Int64 的对比并交换；先对比 new 是否与 old 相同，如果相同就交换并返回 true，如果不同直接返回 false.
func CompareAndSwapInt64(addr *int64, old, new int64) (swapped bool)

// CompareAndSwapUint32 适用于 Uint32 的对比并交换；先对比 new 是否与 old 相同，如果相同就交换并返回 true，如果不同直接返回 false.
func CompareAndSwapUint32(addr *uint32, old, new uint32) (swapped bool)

// CompareAndSwapUint64 适用于 Uint64 的对比并交换；先对比 new 是否与 old 相同，如果相同就交换并返回 true，如果不同直接返回 false.
func CompareAndSwapUint64(addr *uint64, old, new uint64) (swapped bool)

// CompareAndSwapUintptr 适用于 Uintptr 的对比并交换；先对比 new 是否与 old 相同，如果相同就交换并返回 true，如果不同直接返回 false.
func CompareAndSwapUintptr(addr *uintptr, old, new uintptr) (swapped bool)

// CompareAndSwapPointer 适用于 Pointer 的对比并交换；先对比 new 是否与 old 相同，如果相同就交换并返回 true，如果不同直接返回 false.
func CompareAndSwapPointer(addr *unsafe.Pointer, old, new unsafe.Pointer) (swapped bool)

// AddInt32 原子的增加delta 到 *addr 上，并返回相加后的值.
func AddInt32(addr *int32, delta int32) (new int32)

// AddUint32 原子的增加delta 到 *addr 上，并返回相加后的值.
// 如果想用Add方法来减去一个值c，请使用 AddUint32(&x, ^uint32(c-1)).
// 如果要直接减去 x, 执行 AddUint32(&x, ^uint32(0)) 即可.
func AddUint32(addr *uint32, delta uint32) (new uint32)

// AddInt64 原子的增加 delta 到 *addr 上，并返回相加后的值.
func AddInt64(addr *int64, delta int64) (new int64)

// AddUint64 原子的增加 delta 到 *addr 上，并返回相加后的值.
// 如果想用Add方法来减去一个值c，请使用 AddUint64(&x, ^uint64(c-1)).
// 如果要直接减去 x, 执行 AddUint64(&x, ^uint64(0)).
func AddUint64(addr *uint64, delta uint64) (new uint64)

// AddUintptr 原子的增加 delta 到 *addr 上，并返回相加后的值.
func AddUintptr(addr *uintptr, delta uintptr) (new uintptr)

// LoadInt32 原子的获取 *addr 的值.
func LoadInt32(addr *int32) (val int32)

// LoadInt64 原子的获取 *addr 的值.
func LoadInt64(addr *int64) (val int64)

// LoadUint32 原子的获取 *addr 的值.
func LoadUint32(addr *uint32) (val uint32)

// LoadUint64 原子的获取 *addr 的值.
func LoadUint64(addr *uint64) (val uint64)

// LoadUintptr 原子的获取 *addr 的值.
func LoadUintptr(addr *uintptr) (val uintptr)

// LoadPointer 原子的获取 *addr 的值.
func LoadPointer(addr *unsafe.Pointer) (val unsafe.Pointer)

// StoreInt32 原子的赋值 val 到 *addr.
func StoreInt32(addr *int32, val int32)

// StoreInt64 原子的赋值 val 到 *addr.
func StoreInt64(addr *int64, val int64)

// StoreUint32 原子的赋值 val 到 *addr.
func StoreUint32(addr *uint32, val uint32)

// StoreUint64 原子的赋值 val 到 *addr.
func StoreUint64(addr *uint64, val uint64)

// StoreUintptr 原子的赋值 val 到 *addr.
func StoreUintptr(addr *uintptr, val uintptr)

// StorePointer 原子的赋值 val 到 *addr.
func StorePointer(addr *unsafe.Pointer, val unsafe.Pointer)
