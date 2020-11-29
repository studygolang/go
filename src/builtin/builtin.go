// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
    此文件中列出的条目实际上并非定义在 builtin 包中，此处记录只是为了让 godoc 命令能生成语言预定义标识符的文档。
*/
package builtin

// bool 是布尔值（真和假）集合。
type bool bool

// true 和 false 是两个无类型的布尔值。
const (
	true  = 0 == 0 // Untyped bool.
	false = 0 != 0 // Untyped bool.
)

// uint8 是所有8位无符号整数的集合。
// 范围： 0 到 255。
type uint8 uint8

// uint16 是所有16位无符号整数的集合。
// 范围： 0 到 65535。
type uint16 uint16

// uint32 是所有32位无符号整数的集合。
// 范围： 0 到 4294967295。
type uint32 uint32

// uint64 是所有64位无符号整数的集合。
// 范围： 0 到 18446744073709551615。
type uint64 uint64

// int8 是所有8位有符号整数的集合。
// 范围： -128 到 127。
type int8 int8

// int16 是所有16位有符号整数的集合。
// 范围： -32768 到 32767。
type int16 int16

// int32 是所有32位有符号整数的集合。
// 范围： -2147483648 到 2147483647。
type int32 int32

// int64 是所有64位有符号整数的集合。
// 范围： -9223372036854775808 到 9223372036854775807。
type int64 int64

// float32 是所有IEEE-754 32位浮点数的集合。
type float32 float32

// float64 是所有IEEE-754 64位浮点数的集合。
type float64 float64

// complex64 是所有实部和虚部均为32位浮点数的复数集合，
type complex64 complex64

// complex128 是所有实部和虚部均为64位浮点数的复数集合，
type complex128 complex128

// string 是所有8位字节字符的集合，通常但不总表示 UTF-8 编码的文本。string 可以为空，但不能为 nil。
// string 类型的值不可变。
type string string

// int 是有符号整数类型，其大小至少为32位。但它是一个特有类型，而非 int32 的别名。
type int int

// uint 是无符号整数类型，其大小至少为32位。但它是一个特有类型，而非 uint32 的别名。
type uint uint

// uintptr 是一个整数类型，其大小足以容纳任何指针的所有位。
type uintptr uintptr

// byte 是 uint8 的别名，且在所有方面都等同于uint8。按惯例，它用于区分字节和8位无符号整数。
type byte = uint8

// rune 是 int32 的别名，且在所有方面都等同于int32。按惯例，它用于区分字符和整数。
type rune = int32

// iota 是一个预定义标识符，用在常量声明（通常置于括号内）中表示无类型的递增整数。
// iota 索引从零开始。
const iota = 0 // Untyped int.

// nil 是一个预定义标识符，表示指针，通道，函数，接口，映射或切片类型的零值。
var nil Type // Type 必须是指针，通道，函数，接口，映射或切片类型。

// Type 在此处仅用于文档。它是任何 Go 类型的替代，但对于任何给定的函数调用都表示相同的类型。
type Type int

// Type1 在此处仅用于文档。它是任何 Go 类型的替代，但对于任何给定的函数调用都表示相同的类型。
type Type1 int

// IntegerType 在此处仅用于文档。它是 Go 整数类型的替代，包括：int，uint，int8等。
type IntegerType int

// FloatType 在此处仅用于文档。它是 Go 浮点数类型的替代，包括：float32 或 float64。
type FloatType float32

// ComplexType 在此处仅用于文档。它是 Go 复数类型的替代，包括：complex64 或 complex128。
type ComplexType complex64

// 内置函数 append 将元素追加到切片末尾。如果切片容量足够，则将新元素放入并生成新切片。如果容量不够，则将分配一个新数组。
// append 返回值是新切片，因此，有必要将添加元素后的切片存储到原变量中：
//	slice = append(slice, elem1, elem2)
//	slice = append(slice, anotherSlice...)
// 一种特殊情况是，将字符串附加到字节切片, 就像这样：
//	slice = append([]byte("hello "), "world"...)
func append(slice []Type, elems ...Type) []Type

// 内置函数 copy 将元素从源切片复制到目标切片。（一种特殊情况是，它还会将字符串字节复制到字节切片中。）
// 源切片和目标切片可能会存在重叠。
// copy 的返回值是复制的元素个数，该数为 len(src) 和 len(dst) 中的最小值。
func copy(dst, src []Type) int

// 内置函数 delete 从 map m 中删除具有特定 key（m[key]）的元素。如果 m 为 nil 或没有这样的元素，则 delete 什么也不做。
func delete(m map[Type]Type1, key Type)

// 内置函数 len 根据 v 的类型返回其长度：
//	数组：返回 v 的元素数。
//	数组指针：返回 *v 的元素数，即使 v 是 nil。
//	切片或映射： 返回 v 的元素数，若 v 为 nil，则返回0。
//	字符串：返回 v 的字节数。
//	通道：返回通道缓冲区中未读元素个数。若 v 是 nil，则 len(v) 返回0。
// 对于某些参数，例如字符串或简单的数组表达式，返回值可以是常量。详细信息请参见 Go 语言规范“长度和容量”一节。
func len(v Type) int

// 内置函数 cap 根据 v 的类型返回其容量：
//	数组：返回 v 的元素数，与 len(v) 返回值相同。
//	数组指针：返回 *v 的元素数，与 len(v) 返回值相同。
//	切片： 返回切片可利用空间的最大值。如果 v 为 nil，则 cap(v) 返回0。
//  通道：以元素大小为单位返回通道缓冲区容量。如果 v 为 nil，则 cap(v) 返回0。
// 对于某些参数，例如简单的数组表达式，返回值可以是常量。详细信息请参见 Go 语言规范的“长度和容量”一节。
func cap(v Type) int

// 内置函数 make 仅用于分配并初始化切片、映射、通道类型的对象。和 new 函数一样的是，make 第一个参数是类型，而非值；和 new 不同的是，make 的返回类型与其参数类型相同，而非指向对象的指针。具体结果取决于参数类型：
// 切片：第二个参数用于指定切片长度，此时切片的容量等于其长度。可再提供一整数参数以指定不同的容量，该整数不得小于其长度。例如，make([]int，0，10) 分配了一个大小为10的基础数组，并返回一个长度为0，容量为10的切片，该切片底层为数组。
// 映射：为空映射分配足够的空间来容纳指定数量的元素。元素数量可以省略，在这种情况下会分配一较小的初始空间。
// 通道：使用指定的大小初始化通道缓冲区。如果为零或忽略，则该通道无缓冲。
func make(t Type, size ...IntegerType) Type

// 内置函数 new 用于分配内存。第一个参数不是值而是类型，返回值为指向该类型新分配对象零值的指针。
func new(Type) *Type

// 内置函数 complex 使用两浮点数来构造复数。复数实部和虚部必须具有相同的类型，要么都是float32，要么都是float64，返回值为对应类型（complex64 对应 float32，complex128 对应 float64）。
func complex(r, i FloatType) ComplexType

// 内置函数 real 返回复数 c 的实数部分，返回值是和复数 c 对应的浮点数类型。
func real(c ComplexType) FloatType

// 内置函数 imag 返回复数 c 的虚数部分，返回值是和复数 c 对应的浮点数类型。
func imag(c ComplexType) FloatType

// 内置函数 close 用于关闭通道，通道类型必须为收发型或发送型。close 函数只能由发送方执行，而不能由接收方执行。close 函数在接收方收到最后一个值后会关闭通道。
// 当从已关闭通道 c 中接收到最后一个值后，再从 c 中接收值也都会成功而不会阻塞，此时返回值是通道元素类型的零值。如下形式：
//	x, ok := <-c
// 会将 ok 设置为 false。
func close(c chan<- Type)

// 内置函数 panic 会停止当前正常执行的 goroutine。当函数 F 调用 panic 时，F 立即停止执行。F推迟执行的所有函数都将以常规方式运行，然后 F 返回其调用方。对调用者 G 而言，调用 F 就像调用 panic 一样，G 也会终止执行但推迟执行的函数还会继续执行。
// 这个流程会持续到 goroutine 中的所有函数以逆序执行并停止为止。此时，程序以非零退出码终止。这些函数终止的序列合称为 panicking，可以通过内置的函数 recover 控制。
func panic(v interface{})

// 内置函数 recover 使程序能管理 panicking goroutine 的行为。在延迟函数（但不是由它调用的函数）中执行 recover 会获取传递给 panic 的错误值并恢复正常执行。如果在延迟函数之外调用 recover，它将不能停止 panicking。在这种情况下，goroutine 没有发生 panicking，或是提供给 panic 的参数为 nil，recover 都返回 nil。因此，recover 的返回值将报告 goroutine 是否发生 panicking。
func recover() interface{}

// 内置函数 print 以特定实现方式格式化其参数，并将结果写入标准错误。print 在引导和调试阶段很有用。
// Go 语言不保证未来会保留该函数。
func print(args ...Type)

// 内置函数 println 以特定实现方式格式化其参数，并将结果写入标准错误。println 会在参数之间添加空格，并在末尾添加换行符。println 在引导和调试阶段很有用。
// Go 语言不保证未来会保留该函数。
func println(args ...Type)

// 内置接口类型 error 通常用于表示错误，若为 nil 则表示没有错误。
type error interface {
	Error() string
}
