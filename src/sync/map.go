// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
	"unsafe"
)

// Map 类似于map[interface{}]interface{}
// 但是是并发安全的，无需额外的锁的支持.
// 支持的方法 Loads, stores, and deletes .
//
// Map 是一个特殊的类型. 大多数场景下推荐使用原始的map配合锁来进行协作,
// 以提高类型的安全性以及更易于维护.
//
// Sync.map 针对两种场景进行了优化
// (1) 当给定的集合是一个读多写少的场景时,比如缓存
// (2) 当多个goroutine去读写、甚至覆盖完全不想交的条目时
// 在这两种场景下，Sync.map 比使用原生的map配合锁的方式可以明显的减少锁的竞争
//
// Map不需要初始化，零值时即可立即使用，但是第一次使用后，不允许复制
type Map struct {
	mu Mutex

	// 读取read中的
	//
	// read 这个字段在加载的时候是线程安全的，但是如果要修改read字段需要持有锁才可以
	//
	// read 集合中存储的数据并发更新时不需要持有锁
	// 如果更新已经删除的条目需要复制到 dirty map中且需要持有锁.
	read atomic.Value // 只读

	// dirty 集合的数据条目操作时不需持有锁
	// 为了确保尽可能的快从dirty map转到read map中,
	// dirty map 保存了所有read map中未删除的条目.
	//
	// 已经删除的条目不会存储在 dirty map. 
	// 一个已经删除的条目在clean map中必须被标记未清除状态，
	// 然后新增一个值保存时，可以利用该条目并增加到dirty map中.
	//
	// 如果dirty map是空的,下一次的写入会从 clean map中进行浅拷贝（除掉老旧的条目），
	// 来初始化dirty map。
	dirty map[interface{}]*entry

	// misses 存储的是从最后一次更新read map到目前为止来判断条目是否保存在dirty map中的次数
	//
	// 一旦 miss的次数超过dirty map的大小，那么就会将dirty map 提到为read map（未修改状态），
	// 并且下一次存储将会重新初始化dirty map.
	misses int
}

// readOnly 是一个只读的 struct，原子的保存在 Map.read 字段.
type readOnly struct {
	m       map[interface{}]*entry
	amended bool // 如果dirty map中保存了一些 m 中没有的条目，那么为 true.
}

// expunged 是一个指针，标记从dirty map中已经删除的条目
var expunged = unsafe.Pointer(new(interface{}))

// entry 是 map 和特定key的对应的插槽（存储位置）.
type entry struct {
	// p points to the interface{} value stored for the entry.
	//
	// If p == nil, the entry has been deleted and m.dirty == nil.
	//
	// If p == expunged, the entry has been deleted, m.dirty != nil, and the entry
	// is missing from m.dirty.
	//
	// Otherwise, the entry is valid and recorded in m.read.m[key] and, if m.dirty
	// != nil, in m.dirty[key].
	//
	// An entry can be deleted by atomic replacement with nil: when m.dirty is
	// next created, it will atomically replace nil with expunged and leave
	// m.dirty[key] unset.
	//
	// An entry's associated value can be updated by atomic replacement, provided
	// p != expunged. If p == expunged, an entry's associated value can be updated
	// only after first setting m.dirty[key] = e so that lookups using the dirty
	// map find the entry.
	p unsafe.Pointer // *interface{}
}

func newEntry(i interface{}) *entry {
	return &entry{p: unsafe.Pointer(&i)}
}

// Load 从map中加载一个key的值
// ok 表示这个key是否找到一个对应的值.
func (m *Map) Load(key interface{}) (value interface{}, ok bool) {
	read, _ := m.read.Load().(readOnly)
	e, ok := read.m[key]
	if !ok && read.amended {
		m.mu.Lock()
		// Avoid reporting a spurious miss if m.dirty got promoted while we were
		// blocked on m.mu. (If further loads of the same key will not miss, it's
		// not worth copying the dirty map for this key.)
		read, _ = m.read.Load().(readOnly)
		e, ok = read.m[key]
		if !ok && read.amended {
			e, ok = m.dirty[key]
			// Regardless of whether the entry was present, record a miss: this key
			// will take the slow path until the dirty map is promoted to the read
			// map.
			m.missLocked()
		}
		m.mu.Unlock()
	}
	if !ok {
		return nil, false
	}
	return e.load()
}

func (e *entry) load() (value interface{}, ok bool) {
	p := atomic.LoadPointer(&e.p)
	if p == nil || p == expunged {
		return nil, false
	}
	return *(*interface{})(p), true
}

// Store 保存一个指定key的value
func (m *Map) Store(key, value interface{}) {
	read, _ := m.read.Load().(readOnly)
	if e, ok := read.m[key]; ok && e.tryStore(&value) {
		return
	}

	m.mu.Lock()
	read, _ = m.read.Load().(readOnly)
	if e, ok := read.m[key]; ok {
		if e.unexpungeLocked() {
			// The entry was previously expunged, which implies that there is a
			// non-nil dirty map and this entry is not in it.
			m.dirty[key] = e
		}
		e.storeLocked(&value)
	} else if e, ok := m.dirty[key]; ok {
		e.storeLocked(&value)
	} else {
		if !read.amended {
			// We're adding the first new key to the dirty map.
			// Make sure it is allocated and mark the read-only map as incomplete.
			m.dirtyLocked()
			m.read.Store(readOnly{m: read.m, amended: true})
		}
		m.dirty[key] = newEntry(value)
	}
	m.mu.Unlock()
}

// tryStore 尝试存储一个值如果条目没有删除.
//
// 如果这个条目被删除了，返回false并不会修改这个条目
func (e *entry) tryStore(i *interface{}) bool {
	for {
		p := atomic.LoadPointer(&e.p)
		if p == expunged {
			return false
		}
		if atomic.CompareAndSwapPointer(&e.p, p, unsafe.Pointer(i)) {
			return true
		}
	}
}

// unexpungeLocked 确保条目未被标记为删除.
//
// 如果该条目已经删除，那么必须先添加到dirty map再释放m.mu锁.
func (e *entry) unexpungeLocked() (wasExpunged bool) {
	return atomic.CompareAndSwapPointer(&e.p, expunged, nil)
}

// storeLocked 直接存储一个值到map中.
//
// 必须知道该条目不允许被删除.
func (e *entry) storeLocked(i *interface{}) {
	atomic.StorePointer(&e.p, unsafe.Pointer(i))
}

// LoadOrStore 如果该key存在则直接返回value.
// 否则，保存value并返回value.
// loaded 为true表示存在值，false表示保存值成功
func (m *Map) LoadOrStore(key, value interface{}) (actual interface{}, loaded bool) {
	// Avoid locking if it's a clean hit.
	read, _ := m.read.Load().(readOnly)
	if e, ok := read.m[key]; ok {
		actual, loaded, ok := e.tryLoadOrStore(value)
		if ok {
			return actual, loaded
		}
	}

	m.mu.Lock()
	read, _ = m.read.Load().(readOnly)
	if e, ok := read.m[key]; ok {
		if e.unexpungeLocked() {
			m.dirty[key] = e
		}
		actual, loaded, _ = e.tryLoadOrStore(value)
	} else if e, ok := m.dirty[key]; ok {
		actual, loaded, _ = e.tryLoadOrStore(value)
		m.missLocked()
	} else {
		if !read.amended {
			// We're adding the first new key to the dirty map.
			// Make sure it is allocated and mark the read-only map as incomplete.
			m.dirtyLocked()
			m.read.Store(readOnly{m: read.m, amended: true})
		}
		m.dirty[key] = newEntry(value)
		actual, loaded = value, false
	}
	m.mu.Unlock()

	return actual, loaded
}

// tryLoadOrStore 原子的加载一个值，如果该条目没有被删除
//
// 如果条目被删除了, tryLoadOrStore 不会修改这个条目 并 返回ok=false
func (e *entry) tryLoadOrStore(i interface{}) (actual interface{}, loaded, ok bool) {
	p := atomic.LoadPointer(&e.p)
	if p == expunged {
		return nil, false, false
	}
	if p != nil {
		return *(*interface{})(p), true, true
	}

	// Copy the interface after the first load to make this method more amenable
	// to escape analysis: if we hit the "load" path or the entry is expunged, we
	// shouldn't bother heap-allocating.
	ic := i
	for {
		if atomic.CompareAndSwapPointer(&e.p, nil, unsafe.Pointer(&ic)) {
			return i, false, true
		}
		p = atomic.LoadPointer(&e.p)
		if p == expunged {
			return nil, false, false
		}
		if p != nil {
			return *(*interface{})(p), true, true
		}
	}
}

// LoadAndDelete 删除一个key，如果该key存在会返回它的value.
// loaded 表示当前的这个key是否存在.
func (m *Map) LoadAndDelete(key interface{}) (value interface{}, loaded bool) {
	read, _ := m.read.Load().(readOnly)
	e, ok := read.m[key]
	if !ok && read.amended {
		m.mu.Lock()
		read, _ = m.read.Load().(readOnly)
		e, ok = read.m[key]
		if !ok && read.amended {
			e, ok = m.dirty[key]
			delete(m.dirty, key)
			// Regardless of whether the entry was present, record a miss: this key
			// will take the slow path until the dirty map is promoted to the read
			// map.
			m.missLocked()
		}
		m.mu.Unlock()
	}
	if ok {
		return e.delete()
	}
	return nil, false
}

// Delete 删除指定key的条目.
func (m *Map) Delete(key interface{}) {
	m.LoadAndDelete(key)
}

func (e *entry) delete() (value interface{}, ok bool) {
	for {
		p := atomic.LoadPointer(&e.p)
		if p == nil || p == expunged {
			return nil, false
		}
		if atomic.CompareAndSwapPointer(&e.p, p, nil) {
			return *(*interface{})(p), true
		}
	}
}

// Range 调用 f 并依次传入map中的每一个key和value.
// 如果 f 返回 false, 会停止此次循环遍历.
//
// Range 在遍历期间可以保证每个key只会被遍历到一次，但是如果在遍历的期间同时对该key进行了
// 修改或者删除，那么不能保证该value值的可靠性.
//
// Range 的时间复杂度是 O(N) 不管中间是否中断，即f 返回 false.
func (m *Map) Range(f func(key, value interface{}) bool) {
	// 我们需要在调用Range时遍历所有的key.
	// 如 read.amended 是false, 那 read.m 里面就是所有的key 
	// 同时也不需要去持有锁来进行操作.
	read, _ := m.read.Load().(readOnly)
	if read.amended {
		// m.dirty 包含了 read.m中没有的key. 
		// 持有锁后立即将m.dirty 提升到read中
		// 因为既然range了，那么肯定会miss大于dirty map
		m.mu.Lock()
		read, _ = m.read.Load().(readOnly)
		if read.amended {
			read = readOnly{m: m.dirty}
			m.read.Store(read)
			m.dirty = nil
			m.misses = 0
		}
		m.mu.Unlock()
	}

	for k, e := range read.m {
		v, ok := e.load()
		if !ok {
			continue
		}
		if !f(k, v) {
			break
		}
	}
}

func (m *Map) missLocked() {
	m.misses++
	if m.misses < len(m.dirty) {
		return
	}
	m.read.Store(readOnly{m: m.dirty})
	m.dirty = nil
	m.misses = 0
}

func (m *Map) dirtyLocked() {
	if m.dirty != nil {
		return
	}

	read, _ := m.read.Load().(readOnly)
	m.dirty = make(map[interface{}]*entry, len(read.m))
	for k, e := range read.m {
		if !e.tryExpungeLocked() {
			m.dirty[k] = e
		}
	}
}

func (e *entry) tryExpungeLocked() (isExpunged bool) {
	p := atomic.LoadPointer(&e.p)
	for p == nil {
		if atomic.CompareAndSwapPointer(&e.p, nil, expunged) {
			return true
		}
		p = atomic.LoadPointer(&e.p)
	}
	return p == expunged
}
