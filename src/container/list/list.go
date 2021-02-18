// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// list 包实现了双向链表。
//
// 遍历一个链表（这里 l 的类型是 *List）：
//	for e := l.Front(); e != nil; e = e.Next() {
//		// do something with e.Value
//	}
//
package list

// Element 是链表中的一个元素。
type Element struct {
	// 在双链表元素之间的 next 和 previous 指针。 
	// 为了简化实现，链表 l 内部被实现成环(ring)，这样，＆l.root 既是最后一个链表元素
	// (l.Back()) 的下一个元素，又是链表中第一个元素 (l.Front()) 的上一个元素。
	next, prev *Element

	// 该元素所属的链表。
	list *List

	// 值被存储在这个元素中。
	Value interface{}
}

// Next 返回下一个链表元素或者 nil。
func (e *Element) Next() *Element {
	if p := e.next; e.list != nil && p != &e.list.root {
		return p
	}
	return nil
}

// Prev 返回前一个链表元素或者 nil。
func (e *Element) Prev() *Element {
	if p := e.prev; e.list != nil && p != &e.list.root {
		return p
	}
	return nil
}

// List 表示一个双链表。
// List 的零值是可以使用的空链表。
type List struct {
	root Element // 链表的哨兵元素， 只有 &root, root.prev, and root.next 被使用
	len  int     // 当前链表的长度，不包括哨兵元素
}

// Init 初始化或者清空链表 l。
func (l *List) Init() *List {
	l.root.next = &l.root
	l.root.prev = &l.root
	l.len = 0
	return l
}

// New 返回一个初始化的链表。
func New() *List { return new(List).Init() }

// Len 返回链表 l 中的元素数量。
// 复杂度是 O(1)。
func (l *List) Len() int { return l.len }

// Front 返回链表 l 的第一个元素，链表为空时返回 nil。
func (l *List) Front() *Element {
	if l.len == 0 {
		return nil
	}
	return l.root.next
}

// Back 返回链表中最后一个元素，链表为空时返回 nil。
func (l *List) Back() *Element {
	if l.len == 0 {
		return nil
	}
	return l.root.prev
}

// lazyInit lazily initializes a zero List value.
func (l *List) lazyInit() {
	if l.root.next == nil {
		l.Init()
	}
}

// insert inserts e after at, increments l.len, and returns e.
func (l *List) insert(e, at *Element) *Element {
	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e
	e.list = l
	l.len++
	return e
}

// insertValue is a convenience wrapper for insert(&Element{Value: v}, at).
func (l *List) insertValue(v interface{}, at *Element) *Element {
	return l.insert(&Element{Value: v}, at)
}

// remove 从 e 所在的链表中移除 e，减小 l.len，并返回 e。
func (l *List) remove(e *Element) *Element {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.next = nil // 避免内存泄漏
	e.prev = nil // 避免内存泄漏
	e.list = nil
	l.len--
	return e
}

// move 移动 e 到 at 的下一个元素（at.next = e），并返回 e。
func (l *List) move(e, at *Element) *Element {
	if e == at {
		return e
	}
	e.prev.next = e.next
	e.next.prev = e.prev

	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e

	return e
}

// 如果 e 是链表 l 的元素，Remove 移除元素 e。
// 它返回元素 e 的值 e.Value。
// 元素必须不为 nil。
func (l *List) Remove(e *Element) interface{} {
	if e.list == l {
		// 如果 e.list == l, 当 e 被插入时 l 必须已经初始化
		// 否则 l.remove 将会崩溃当 l == nil 时
		l.remove(e)
	}
	return e.Value
}

// PushFront 在链表头部插入一个值为 v 的新元素 e， 并返回 e。
func (l *List) PushFront(v interface{}) *Element {
	l.lazyInit()
	return l.insertValue(v, &l.root)
}

// PushBack 在链表尾部插入一个值为 v 的新元素 e， 并返回 e。
func (l *List) PushBack(v interface{}) *Element {
	l.lazyInit()
	return l.insertValue(v, l.root.prev)
}

// InsertBefore 在 mark 之前插入一个值为 v 的新元素 e，并返回 e。
// 如果 mark 不是 l 的元素，链表不会被修改。
// mark 必须不为 nil。
func (l *List) InsertBefore(v interface{}, mark *Element) *Element {
	if mark.list != l {
		return nil
	}
	// 看 List.Remove 处关于 l 初始化的注释
	return l.insertValue(v, mark.prev)
}

// InsertAfter 在 mark 之后插入一个值为 v 的新元素 e，并返回 e。
// 如果 mark 不是 l 的元素，链表不会被修改。
// mark 必须不为 nil。
func (l *List) InsertAfter(v interface{}, mark *Element) *Element {
	if mark.list != l {
		return nil
	}
	// 看 List.Remove 处关于 l 初始化的注释
	return l.insertValue(v, mark)
}

// MoveToFront 将元素 e 移动到列表 l 的前面。
// 如果 e 不是 l 的一个元素，链表不会被修改。
// 元素必须不为 nil。
func (l *List) MoveToFront(e *Element) {
	if e.list != l || l.root.next == e {
		return
	}
	// 看 List.Remove 处关于 l 初始化的注释
	l.move(e, &l.root)
}

// MoveToBack 将元素 e 移动到列表 l 的后面。
// 如果 e 不是 l 的一个元素，链表不会被修改。
// 元素必须不为 nil。
func (l *List) MoveToBack(e *Element) {
	if e.list != l || l.root.prev == e {
		return
	}
	// 看 List.Remove 处关于 l 初始化的注释
	l.move(e, l.root.prev)
}

// MoveBefore 移动元素 e 到标记（mark参数指定）的位置之前。
// 如果 e 或者 mark 不是 l 的一个元素，或者 e == mark，链表不会被修改。
// 元素和 mark 必须不为 nil。
func (l *List) MoveBefore(e, mark *Element) {
	if e.list != l || e == mark || mark.list != l {
		return
	}
	l.move(e, mark.prev)
}

// MoveAfter 移动元素 e 到标记（mark参数指定）的位置之后。
// 如果 e 或者 mark 不是 l 的一个元素，或者 e == mark，链表不会被修改。
// 元素和 mark 必须不为 nil。
func (l *List) MoveAfter(e, mark *Element) {
	if e.list != l || e == mark || mark.list != l {
		return
	}
	l.move(e, mark)
}

// PushBackList 在链表 l 后面插入另一个链表的拷贝。
// 链表 l 和链表 other 可能是相同的。 它们不能是 nil。
func (l *List) PushBackList(other *List) {
	l.lazyInit()
	for i, e := other.Len(), other.Front(); i > 0; i, e = i-1, e.Next() {
		l.insertValue(e.Value, l.root.prev)
	}
}

// PushFrontList 在链表 l 的前面插入另一个链表的拷贝。
// 链表 l 和链表 other 可能是相同的。 它们不能是 nil。
func (l *List) PushFrontList(other *List) {
	l.lazyInit()
	for i, e := other.Len(), other.Back(); i > 0; i, e = i-1, e.Prev() {
		l.insertValue(e.Value, &l.root)
	}
}
