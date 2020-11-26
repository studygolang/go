// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
    unsafe包中包含绕过go类型安全限制的操作，导入unsafe的其他包可能不具可移植性，并且不受Go 1兼容性准则的保护。
*/
package unsafe

// 此处ArbitraryType仅出于文档目的，实际上不是unsafe包的一部分，它表示任意Go表达式的类型。
type ArbitraryType int

// Pointer表示指向任意类型的指针，Pointer有四种特有操作：
//  -任何类型的指针都可以转换为Pointer。
//  -Pointer可以转换为任何类型的指针。
//  -uintptr可以转换为Pointer。
//  -Pointer可以转换为uintptr。
// 因此，Pointer允许程序绕过类型系统并读写内存，使用时应格外小心。
//
// 以下的Pointer使用模式才是合法的。不按这些模式使用Pointer的代码目前可能失效了，或者将来可能失效，甚至下面的有些模式也带有重要警告。
//
// 运行“ go vet”可以帮助查找不符合这些模式的Pointer用法，但是“ go vet”没输出警告不能保证代码就是合法的。
//
// (1) 将*T1转换为Pointer再转为*T2。
// 
// 假定T2不大于T1，并且两个共享内存，则此转换允许将一种类型的数据重新解释为另一种类型的数据。一个例子是math.Float64bits的实现：
//
//	func Float64bits(f float64) uint64 {
//		return *(*uint64)(unsafe.Pointer(&f))
//	}
// 
// (2) 将Pointer转换为uintptr（不再转回Pointer）。
//
// 将Pointer转换为uintptr会得到所指对象内存地址（整数），uintpt的这种用法多是为了打印值。
//
// 通常，将uintptr转为Pointer是非法的。
//
// uintptr是整数，而不是引用。将Pointer转换为uintptr会创建一个去除指针语义的整数值。即使uintptr拥有某个对象的地址，垃圾回收器也不会在对象移动时更新该uintptr的值，而uintptr也不会阻止所指对象被回收。
//
// 下面列举的从uintptr到Pointer的转换是唯一合法的。
//
// (3) 将Pointer转为uintptr，进行数学运算，最后转回Pointer。
//
// 如果p指向已分配内存对象，则可转换p为uintptr，加上一个偏移量，最后将其转换回Pointer。
//
//	p = unsafe.Pointer(uintptr(p) + offset)
//
// 此模式最常见的用法是访问结构体中字段或数组中的元素：
//
//	// 等效于 f := unsafe.Pointer(&s.f)
//	f := unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + unsafe.Offsetof(s.f))
//
//	// 等效于 e := unsafe.Pointer(&x[i])
//	e := unsafe.Pointer(uintptr(unsafe.Pointer(&x[0])) + i*unsafe.Sizeof(x[0]))
//
// 以这种方式给指针添加和减去一个偏移量都是合法的。使用＆^舍入指针（通常用于对齐）也是合法的。在所有情况下，转换结果都必须指向原分配对象。
//
// 与C语言不同，将指针指向其对象末尾是非法的：
//
//	// INVALID: 指向分配空间之外。
//	var s thing
//	end = unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + unsafe.Sizeof(s))
//
//	// INVALID: 指向分配空间之外。
//	b := make([]byte, n)
//	end = unsafe.Pointer(uintptr(unsafe.Pointer(&b[0])) + uintptr(n))
//
// 请注意，两次转换必须出现在同一个表达式中，并且算术运算也只能出现在该表达式中：
//
//	// INVALID: 在转回Pointer前，uintptr不能存储在变量中。
//	u := uintptr(p)
//	p = unsafe.Pointer(u + offset)
//
// 请注意，指针必须指向已分配的对象，不能指向nil。
//
//	// INVALID: nil指针转换。
//	u := unsafe.Pointer(nil)
//	p := unsafe.Pointer(uintptr(u) + offset)
//
// (4) 调用syscall.Syscall时将Pointer转换为uintptr。
//
// syscall包中的Syscall函数将其uintptr参数直接传递给操作系统，然后操作系统根据调用情况将其中一些参数重新解释为指针。也就是说，系统调用会将某些参数从uintptr隐式地转回指针。
//
// 如果必须将指针参数转换为uintptr，则该转换必须出现在调用表达式中：
//
//	syscall.Syscall(SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(p)), uintptr(n))
//
// 从类型上来看，即使调用过程中对象不再需要了，但编译器还是会确保Pointer（已转换为uintptr）作为参数传递给汇编函数时，它所指向的对象不会改变和移动，直到调用完成。
//
// 为使编译器识别这种模式，转换必须出现在参数列表中：
//
//	// INVALID: 系统调用中，隐式地转回Pointer前，uintptr不能存储在变量中
//	u := uintptr(unsafe.Pointer(p))
//	syscall.Syscall(SYS_READ, uintptr(fd), u, uintptr(n))
//
// (5) 将reflect.Value.Pointer或reflect.Value.UnsafeAddr的值从uintptr转换为Pointer。
//
// reflect包的Value.Pointer和Value.UnsafeAddr方法返回的是uintptr而不是Pointer，这样调用者可将返回值改为任意类型，而无需导入unsafe包。但是，这种返回值很不稳定，必须在调用后立即在同一表达式中将其转换为Pointer：
//
//	p := (*int)(unsafe.Pointer(reflect.ValueOf(new(int)).Pointer()))
//
// 与上述情况一样，在转换之前存储返回值是非法的：
//
//	// INVALID: 转换回Pointer之前，uintptr不能存储在变量中。
//	u := reflect.ValueOf(new(int)).Pointer()
//	p := (*int)(unsafe.Pointer(u))
//
// (6) 将reflect.SliceHeader或reflect.StringHeader字段与指针进行相互转换。
//
// 与前面的情况一样，反射数据结构SliceHeader和StringHeader将字段Data声明为uintptr，这样调用者不导入unsafe包也可将返回值转换为任意类型。但这意味着SliceHeader和StringHeader仅在解释实际切片或字符串值时才合法。
//
//	var s string
//	hdr := (*reflect.StringHeader)(unsafe.Pointer(&s)) // case 1
//	hdr.Data = uintptr(unsafe.Pointer(p))              // case 6 (this case)
//	hdr.Len = n
//
// 在这种用法中，hdr.Data实际上是引用字符串Header中基础指针的替代方法，而不是uintptr变量本身。
//
// 通常，reflect.SliceHeader和reflect.StringHeader只能以*reflect.SliceHeader和*reflect.StringHeader的形式指向切片或字符串，而不能单独存在，程序中不应声明或分配这两种类型的变量。
//
//	// INVALID: 直接声明的Header不会作为保存数据的引用。
//	var hdr reflect.StringHeader
//	hdr.Data = uintptr(unsafe.Pointer(p))
//	hdr.Len = n
//	s := *(*string)(unsafe.Pointer(&hdr)) // p 可能已不存在
//
type Pointer *ArbitraryType

// Sizeof接收任何类型的表达式x并返回假设变量v的字节大小，v是通过类似var v = x声明的假设变量。该大小不包括x可能引用的其他内存。例如，若x为切片，则Sizeof返回切片描述符的大小，而不是该切片所引用的内存大小。Sizeof的返回值是常数。
func Sizeof(x ArbitraryType) uintptr

// Offsetof返回结构体x的字段偏移量，其格式必须为structValue.field。换句话说，它返回结构体起始处到字段起始处之间的字节数。Offsetof的返回值是常数。
func Offsetof(x ArbitraryType) uintptr

// Alignof接收任何类型的表达式x并返回假设变量v的对齐方式，v是通过类似var v = x声明的假设变量。它是一个最大值m，因此v的地址模m始终为零。Alignof与reflect.TypeOf(x).Align()返回的值相同。若变量s是结构体类型，而f是该结构体中字段，则Alignof(s.f)将返回该字段的对齐方式。这种情况与reflect.TypeOf(s.f).FieldAlign()的返回值相同。Alignof的返回值是常数。
func Alignof(x ArbitraryType) uintptr
