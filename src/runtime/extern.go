// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
runtime 包含与 Go 的运行时系统交互的操作，例如控制 goroutines 的函数。
它还包括 reflect 包使用的低级类型信息；
有关 run-time 类型系统的可编程接口，请参见 reflect 的文档。

Environment Variables

下列环境变量( $name 或 %name%，取决于主机操作系统)控制 Go 程序的运行时行为。
不同版本的含义和用途可能会有所不同。

GOGC 变量设置初始垃圾收集目标百分比。当新分配的数据与上次收集后剩余的实时数据的比率达到此百分比时，将触发收集。
默认情况下 GOGC=100。设置 GOGC=off 会完全禁用垃圾收集器(garbage collector)。
runtime/debug 包的 SetGCPercent 函数允许在运行时改变这个百分比的值。
请参见 https://golang.org/pkg/runtime/debug/#SetGCPercent。

GODEBUG 变量控制运行时中的调试变量(debugging variables)。
它是一个逗号分隔的成对出现的 name=val 组成列表，用于设置这些指定的变量:

	allocfreetrace: 设置 allocfreetrace=1 会导致每个分配都被分析，并在每个对象的分配上打印一个堆栈跟踪。

	clobberfree: 设置 clobberfree=1 会导致垃圾收集器(garbage collector)在释放对象时，
	用错误的内容(bad content)来清理对象的内存内容(memory content)。

	cgocheck: 设置 cgocheck=0 将禁用对使用 cgo 错误地将 go 指针传递给非 Go 代码的包的所有检查。
	设置 cgocheck=1(默认值) 可以进行相对便宜的检查，可能会遗漏一些错误。
	设置 cgocheck=2 可以进行昂贵的检查，不会遗漏任何错误，但会导致程序运行速度变慢。

	efence: 设置 efence=1 会导致分配器(allocator)以一种模式运行，
	在这种模式下，每个对象都被分配到一个唯一的页(page)上，并且地址永远不会被回收。

	gccheckmark: 设置 gccheckmark=1 可以在发生 STW(Stop-the-World, 指的是GC事件发生过程中，
	会产生应用程序的停顿。这个停顿称为STW) 时通过执行第二次标记传递来验证垃圾收集器的并发标记阶段。
	如果第二遍找到一个并发标记未找到的可到达对象，垃圾收集器将会 panic。

	gcpacertrace: 设置 gcpacertrace=1 会导致垃圾收集器打印有关 concurrent pacer 内部状态的信息。

	gcshrinkstackoff: 设置 gcshrinkstackoff=1 将禁止将 goroutines 移动到较小的堆栈上。
	在这种模式下，goroutine 的堆栈只能增长。

	gcstopthewrold: 设置 gcstoptheworld=1 将禁用并发垃圾收集，使每个垃圾收集都成为一个 SWT 事件。
	设置 gcstoptheworld=2 还会在垃圾收集完成后禁用并发清理。

	gctrace: 设置 gctrace=1 会导致垃圾收集器在每次收集时发出单行的标准错误，
	汇总收集的内存量和暂停时间。此行的格式可能会有所更改。
	目前，它是这样的：
		gc # @#s #%: #+#+# ms clock, #+#/#/#+# ms cpu, #->#-># MB, # MB goal, # P
	其中字段如下：
		gc #        GC 编号, 每次垃圾收集时递增
		@#s         程序启动后的时间(秒)
		#%          自程序启动后花费在垃圾收集上的时间百分比
		#+...+#     在垃圾收集阶段 wall-clock/CPU 时间
		#->#-># MB  垃圾回收开始时、垃圾回收结束时和实时堆的堆大小
		# MB goal   目标堆大小
		# P         使用的处理器数量
	这些阶段是 stop-the-world(STW) 扫描终止、并发标记和扫描以及STW标记终止。
	标记/扫描的中央处理器时间分为辅助时间(根据分配执行的垃圾收集)、后台垃圾收集时间和空闲垃圾收集时间。
	如果该行以“(forced)”结尾，则该垃圾收集是由 runtime.GC() 强制调用的。

	inittrace: 设置 inittrace=1 会导致运行时为每个具有 init work 的包发出单行的标准错误，
	汇总执行时间和内存分配。对于作为插件加载的一部分执行的 init 和没有用户定义和编译器生成的 
	init 工作的包，不打印任何信息。
	此行的格式可能会有所更改。目前，它是这样的:
		init # @#ms, # ms clock, # bytes, # allocs
	其中字段如下:
		init #      包的名称
		@# ms       从程序启动后到 init 启动的时间(毫秒)
		# clock     包初始化工作的 wall-clock 时间
		# bytes     堆上分配的内存
		# allocs    堆分配的数量

	madvdontneed: 设置 madvdontneed=0 将在Linux上使用 MADV_FREE 而不是 
	MADV_DONTNEED 来为内核返回内存。这样效率更高，但是意味着 RSS 数量只有在
	OS 内存压力大的时候才会下降。

	memprofilerate: 设置 memprofilerate=X 将更新 runtime.MemprofileRate 的值。
	设置为0时，禁用内存分析。有关默认值，请参考 MemProfileRate 的描述。

	invalidptr: 如果在 pointer-typed 的位置发现无效的指针值(例如 invalidptr=1(默认值)
	会导致垃圾收集器和堆栈复制器崩溃。设置 invalidptr=0 将禁用此检查。
	这只能作为诊断有问题代码的临时解决方法。
	真正的解决办法是不在 pointer-typed 的位置存储整数。

	sbrk: 设置 sbrk=1 将内存分配器和垃圾收集器替换为简单的分配器，
	它从操作系统获取内存，并且从不回收任何内存。

	scavenge: scavenge=1 启用堆清除程序的调试模式。

	scavtrace: 设置scavtrace=1会导致运行时发出一个单行的标准错误，大约每个 GC 周期一次，
	汇总清除程序完成的工作量以及返回给操作系统的内存总量和对物理内存利用率的估计。
	该行的格式可能会更改，但目前是:
		scav # # KiB work, # KiB total, #% util
	其中字段如下:
		scav #       清除循环编号
		# KiB work   自最后一行以来返回给操作系统的内存量
		# KiB total  返回给操作系统的内存总量
		#% util      正在使用的所有未清理内存的一部分。如果该行以“(forced)”结尾，
		则 scavenging 会通过调用 debug.FreeOSMemory() 强制清理。

	scheddetail: 设置 schedtrace=X 和 scheddetail=1 会导致调度程序每X毫秒发出详细的多行信息，
	描述调度程序、处理器、线程和 goroutines 的状态。

	schedtrace: 设置 schedtrace=X 会导致调度程序每隔X毫秒发出一行标准错误，汇总调度程序的状态。

	tracebackancestors: 设置 tracebackancestors=N 会使用创建 goroutines 的堆栈扩展回溯
	其中 N 限制了要报告的祖先 goroutines 的数量。这也扩展了 runtime.Stack 返回的信息。
	祖先的 goroutine IDs 指的是 gorotine 在创建时的 ID；该 ID 有可能被重新用于其他 goroutine。
	将N设置为0将不会报告祖先信息。

	asyncpreemptoff: asyncpreemptoff=1 禁用基于信号的异步 goroutine 抢占。
	这使得一些循环长时间不可抢占，这可能会延迟 GC 和 goroutine 调度。
	这对于调试 GC 问题很有用，因为它还禁用了用于异步抢占的 goroutines 的保守堆栈扫描。

net、net/http、crypto/tls 这些包也涉及 GODEBUG 中的调试变量(debugging variables)。
详细内容请参见这些包的文档。

GOMAXPROCS 变量限制了可以同时执行用户级 Go 代码的操作系统线程的数量。
对于以 Go 代码为代表所产生的系统调用，可被阻塞的线程数没有限制；这些不计入 GOMAXPROCS 限制。
这个包的 GOMAXPROCS 函数可查询限制和修改限制。

GORACE 变量配置竞争检测器(race detector)，构建程序时使用 -race 参数。
详细内容请参见 https://golang.org/doc/articles/race_detector.html。

GOTRACEBACK 变量控制当 Go 程序由于未恢复的死机或意外的运行时条件而失败时生成的输出量。
默认情况下，失败会打印当前 goroutine 的堆栈跟踪，省略运行时系统内部的函数，然后以 exit code 2 退出。
如果没有当前的 goroutine 或失败是运行时内部的，则失败会打印所有 goroutine 的堆栈跟踪。
GOTRACEBACK=none 完全省略 goroutine 堆栈跟踪。
GOTRACEBACK=single （默认值）行为如上所述。
GOTRACEBACK=all 为所有 user-created goroutines 添加堆栈跟踪。
GOTRACEBACK=system 类似于 "all" 但为运行时函数添加了栈帧并显示运行时内部创建的 goroutines。
GOTRACEBACK=crash 类似于 "system" 但是以特定于操作系统的方式崩溃，而不是退出。例如，在 Unix 系统上，崩溃引发 SIGABRT 来触发核心转储。
出于历史原因，GOTRACEBACK设置 0、1、2 分别是 none、all 和 system 的同义词。
runtime/debug 包的 SetTraceback 函数允许在运行时增加输出量，但它不能将输出量减少到环境变量指定的值以下。
请参见 https://golang.org/pkg/runtime/debug/#SetTraceback。

GOARCH、GOOS、GOPATH 和 GOROOT 环境变量完成了 Go 环境变量集。它们影响 Go 程序的建立
(参见 https://golang.org/cmd/go 和 https://golang.org/pkg/go/build)。
GOARCH、GOOS 和 GOROOT 是在编译时记录的，并通过这个包中的常量或函数提供，但是它们不影响运行时系统的执行。
*/
package runtime

import "runtime/internal/sys"

// Caller 在调用 goroutine 的堆栈上报告关于函数调用的文件和行号信息。
// 参数 skip 是要递增的堆栈帧数，0 表示调用者的 Caller。
// (由于历史原因，Caller 和 Callers 之间跳过的含义不同。)
// 返回值报告相应调用的文件中的程序计数器、文件名和行号。
// 如果无法 recover 信息，则布尔值 ok 为 false。
func Caller(skip int) (pc uintptr, file string, line int, ok bool) {
	rpc := make([]uintptr, 1)
	n := callers(skip+1, rpc[:])
	if n < 1 {
		return
	}
	frame, _ := CallersFrames(rpc).Next()
	return frame.PC, frame.File, frame.Line, frame.PC != 0
}

// Callers 用调用 goroutine 栈上函数调用的返回程序计数器填充 slice pc。
// skip 参数是在 pc 中记录之前要跳过的堆栈帧数，0 表示 Callers 本身的帧，
// 1 表示 Callers 的调用者。它返回写入 pc 的条目数。
// 
// 要将这些 PCs 转换为符号信息，如函数名和行号，请使用CallersFrames。
// CallersFrames 负责内联函数，并调整返回程序计数器到调用程序计数器。
// 不鼓励直接对返回的 PCs 进行迭代，就像在任何返回的程序片上使用 FuncForPC 一样，
// 因为这些不能解释内联或返回程序计数器调整。
func Callers(skip int, pc []uintptr) int {
	// runtime.callers uses pc.array==nil as a signal
	// to print a stack trace. Pick off 0-length pc here
	// so that we don't let a nil pc slice get to it.
	if len(pc) == 0 {
		return 0
	}
	return callers(skip, pc)
}

// GOROOT 返回 Go 树的根。
// 它使用 GOROOT 环境变量(如果在进程开始时设置的话)，或者在 Go build 过程中使用的根。
func GOROOT() string {
	s := gogetenv("GOROOT")
	if s != "" {
		return s
	}
	return sys.DefaultGoroot
}

// Version 返回 Go 树的版本字符串。
// 它或者是构建时的提交 hash 和 date，或者，如果可能的话，是像 “go1.3” 这样的发布标签。
func Version() string {
	return sys.TheVersion
}

// GOOS 是运行程序的操作系统目标: darwin、freebsd、linux 等当中的一个。
// 要查看 GOOS 和 GOARCH 的可能组合，请运行“go tool dist list”。
const GOOS string = sys.GOOS

// GOARCH 是运行程序的体系结构目标: 386、amd64、arm、s390x 等当中的一个。
const GOARCH string = sys.GOARCH
