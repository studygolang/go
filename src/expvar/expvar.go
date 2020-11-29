// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// expvar 包提供了与公共变量（如服务器中的操作计数器）间的标准化接口，它通过 HTTP 在 /debug/vars 处以JSON格式暴露这些变量。
//
// 设置或修改这些公共变量的操作都是原子性的。
//
// expvar 包除了添加 HTTP handler 外，还注册了以下变量：
//
//	cmdline   os.Args
//	memstats  runtime.Memstats
//
// 有时仅出于注册 HTTP handler 和上述变量时的副作用而导入本包。为了以副作用方式使用本包，请将此包按如下方式导入到程序中：
//	import _ "expvar"
//
package expvar

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Var 是所有导出变量的抽象类型。
type Var interface {
	// String 返回变量的合法JSON值。
	// 返回非法JSON值（例如 time.Time）的各 String 方法不得用作 Var。
	String() string
}

// Int 是一个满足 Var 接口的64位整型变量。
type Int struct {
	i int64
}

func (v *Int) Value() int64 {
	return atomic.LoadInt64(&v.i)
}

func (v *Int) String() string {
	return strconv.FormatInt(atomic.LoadInt64(&v.i), 10)
}

func (v *Int) Add(delta int64) {
	atomic.AddInt64(&v.i, delta)
}

func (v *Int) Set(value int64) {
	atomic.StoreInt64(&v.i, value)
}

// Float 是一个满足 Var 接口的64位浮点型变量。
type Float struct {
	f uint64
}

func (v *Float) Value() float64 {
	return math.Float64frombits(atomic.LoadUint64(&v.f))
}

func (v *Float) String() string {
	return strconv.FormatFloat(
		math.Float64frombits(atomic.LoadUint64(&v.f)), 'g', -1, 64)
}

// Add 把 delta 加到 v。
func (v *Float) Add(delta float64) {
	for {
		cur := atomic.LoadUint64(&v.f)
		curVal := math.Float64frombits(cur)
		nxtVal := curVal + delta
		nxt := math.Float64bits(nxtVal)
		if atomic.CompareAndSwapUint64(&v.f, cur, nxt) {
			return
		}
	}
}

// Set 把 v 设置为 value。
func (v *Float) Set(value float64) {
	atomic.StoreUint64(&v.f, math.Float64bits(value))
}

// Map 是满足 Var 接口的 string-to-Var 映射。
type Map struct {
	m      sync.Map // map[string]Var
	keysMu sync.RWMutex
	keys   []string // sorted
}

// KeyValue 表示 Map 中的单个条目。
type KeyValue struct {
	Key   string
	Value Var
}

func (v *Map) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "{")
	first := true
	v.Do(func(kv KeyValue) {
		if !first {
			fmt.Fprintf(&b, ", ")
		}
		fmt.Fprintf(&b, "%q: %v", kv.Key, kv.Value)
		first = false
	})
	fmt.Fprintf(&b, "}")
	return b.String()
}

// Init 将 Map 中所有键删除。
func (v *Map) Init() *Map {
	v.keysMu.Lock()
	defer v.keysMu.Unlock()
	v.keys = v.keys[:0]
	v.m.Range(func(k, _ interface{}) bool {
		v.m.Delete(k)
		return true
	})
	return v
}

// addKey 更新 v.keys 组成的有序列表。
func (v *Map) addKey(key string) {
	v.keysMu.Lock()
	defer v.keysMu.Unlock()
	// 用插入排序将 key 放入有序的 v.keys。
	if i := sort.SearchStrings(v.keys, key); i >= len(v.keys) {
		v.keys = append(v.keys, key)
	} else if v.keys[i] != key {
		v.keys = append(v.keys, "")
		copy(v.keys[i+1:], v.keys[i:])
		v.keys[i] = key
	}
}

func (v *Map) Get(key string) Var {
	i, _ := v.m.Load(key)
	av, _ := i.(Var)
	return av
}

func (v *Map) Set(key string, av Var) {
	// 在存储值之前，请检查 key 是否为最新值。在调用 LoadOrStore 之前尝试先调用 Load：LoadOrStore 导致即使在 Load 路径上，key 接口也会逃逸。
	if _, ok := v.m.Load(key); !ok {
		if _, dup := v.m.LoadOrStore(key, av); !dup {
			v.addKey(key)
			return
		}
	}

	v.m.Store(key, av)
}

// Add 将 delta 加到存储在 map[key] 中 *Int 值上。
func (v *Map) Add(key string, delta int64) {
	i, ok := v.m.Load(key)
	if !ok {
		var dup bool
		i, dup = v.m.LoadOrStore(key, new(Int))
		if !dup {
			v.addKey(key)
		}
	}

	// 加到 Int，要么忽略。
	if iv, ok := i.(*Int); ok {
		iv.Add(delta)
	}
}

// AddFloat 将 delta 加到存储在 map[key] 中 *Float 值上。
func (v *Map) AddFloat(key string, delta float64) {
	i, ok := v.m.Load(key)
	if !ok {
		var dup bool
		i, dup = v.m.LoadOrStore(key, new(Float))
		if !dup {
			v.addKey(key)
		}
	}

	// 加到 Float，要么忽略。
	if iv, ok := i.(*Float); ok {
		iv.Add(delta)
	}
}

// Delete 从 map 中删除 key。
func (v *Map) Delete(key string) {
	v.keysMu.Lock()
	defer v.keysMu.Unlock()
	i := sort.SearchStrings(v.keys, key)
	if i < len(v.keys) && key == v.keys[i] {
		v.keys = append(v.keys[:i], v.keys[i+1:]...)
		v.m.Delete(key)
	}
}

// Do 为 map 中每个条目调用 f。
// 该 map 在迭代调用过程中会被锁定。但现存条目可并发地更新。
func (v *Map) Do(f func(KeyValue)) {
	v.keysMu.RLock()
	defer v.keysMu.RUnlock()
	for _, k := range v.keys {
		i, _ := v.m.Load(k)
		f(KeyValue{k, i.(Var)})
	}
}

// String 是一个字符串变量且满足 Var 接口。
type String struct {
	s atomic.Value // string
}

func (v *String) Value() string {
	p, _ := v.s.Load().(string)
	return p
}

// String 实现了 Var 接口。要获取未加引号的字符串，请使用 Value 方法。
func (v *String) String() string {
	s := v.Value()
	b, _ := json.Marshal(s)
	return string(b)
}

func (v *String) Set(value string) {
	v.s.Store(value)
}

// Func 通过调用函数并使用JSON格式化返回值实现了 Var 接口。
type Func func() interface{}

func (f Func) Value() interface{} {
	return f()
}

func (f Func) String() string {
	v, _ := json.Marshal(f())
	return string(v)
}

// 所有公开变量。
var (
	vars      sync.Map // map[string]Var
	varKeysMu sync.RWMutex
	varKeys   []string // sorted
)

// Publish 声明一个已命名的导出变量。创建 Vars 时，应从包中的 init 函数中调用 Publish。如果该变量已经注册，则会调用 log.Panic。
func Publish(name string, v Var) {
	if _, dup := vars.LoadOrStore(name, v); dup {
		log.Panicln("Reuse of exported var name:", name)
	}
	varKeysMu.Lock()
	defer varKeysMu.Unlock()
	varKeys = append(varKeys, name)
	sort.Strings(varKeys)
}

// Get 获取一个已命名的导出变量。若该变量尚未注册，则返回 nil。
func Get(name string) Var {
	i, _ := vars.Load(name)
	v, _ := i.(Var)
	return v
}

// 方便创建新的导出变量的函数。

func NewInt(name string) *Int {
	v := new(Int)
	Publish(name, v)
	return v
}

func NewFloat(name string) *Float {
	v := new(Float)
	Publish(name, v)
	return v
}

func NewMap(name string) *Map {
	v := new(Map).Init()
	Publish(name, v)
	return v
}

func NewString(name string) *String {
	v := new(String)
	Publish(name, v)
	return v
}

// Do 为每个导出变量调用 f。
// 全局变量 map 在迭代调用过程中会被锁定，但现存条目可并发地更新。
func Do(f func(KeyValue)) {
	varKeysMu.RLock()
	defer varKeysMu.RUnlock()
	for _, k := range varKeys {
		val, _ := vars.Load(k)
		f(KeyValue{k, val.(Var)})
	}
}

func expvarHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "{\n")
	first := true
	Do(func(kv KeyValue) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}

// Handler 返回 expvar HTTP Handler。
//
// 仅在将 handler 安装在非标准位置时才需要这样做。
func Handler() http.Handler {
	return http.HandlerFunc(expvarHandler)
}

func cmdline() interface{} {
	return os.Args
}

func memstats() interface{} {
	stats := new(runtime.MemStats)
	runtime.ReadMemStats(stats)
	return *stats
}

func init() {
	http.HandleFunc("/debug/vars", expvarHandler)
	Publish("cmdline", Func(cmdline))
	Publish("memstats", Func(memstats))
}
