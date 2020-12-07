// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ring 包实现了循环链表的操作。
package ring

// Ring 是一个循环链表的元素，或者说是环。
// 环没有起点和终点；有一个可以指向任何环中元素的指针作为对整个环的引用。
// 空的环被表示为 Ring 类型的 nil。
// Ring 的零值是只有一个元素的环，该元素的值为 nil。
//
type Ring struct {
	next, prev *Ring
	Value      interface{} // 供用户使用，不受该库的影响
}

func (r *Ring) init() *Ring {
	r.next = r
	r.prev = r
	return r
}

// Next 返回环的下一个元素。 r 必须非空。
func (r *Ring) Next() *Ring {
	if r.next == nil {
		return r.init()
	}
	return r.next
}

// Prev 返回环的上一个元素。 r 必须非空。
func (r *Ring) Prev() *Ring {
	if r.next == nil {
		return r.init()
	}
	return r.prev
}

// Move 在环中向后 (n < 0) 或者向前 (n >= 0) 移动指针 n % r.Len() 次，
// 并返回此时所指的元素。 r 必须非空。
//
func (r *Ring) Move(n int) *Ring {
	if r.next == nil {
		return r.init()
	}
	switch {
	case n < 0:
		for ; n < 0; n++ {
			r = r.prev
		}
	case n > 0:
		for ; n > 0; n-- {
			r = r.next
		}
	}
	return r
}

// New 创建一个有 n 个元素的环。
func New(n int) *Ring {
	if n <= 0 {
		return nil
	}
	r := new(Ring)
	p := r
	for i := 1; i < n; i++ {
		p.next = &Ring{prev: p}
		p = p.next
	}
	p.next = r
	r.prev = p
	return r
}

// Link 将 s 连接到 r 上，以至于 r.Next() 为 s。
// 返回值为 r.Next() 的原来的值。
// r 必须非空。
//
// 如果 r 和 s 指向相同的环，调用 Link 将会移除 r 和 s 之间的元素。
// 移除的元素会形成一个子环，且返回值是那个子环的引用。
// 如果没有元素被移除，返回值仍然是 r.Next() 原来的值。
//
// 如果 r 和 s 指向不同的环，将它们连接起来就会形成一个 s 被插在 r 后的环，
// 插入完成后返回结果指向接着 s 中最后一个元素的元素（返回值为 r.Next 的原来的值）。
//
func (r *Ring) Link(s *Ring) *Ring {
	n := r.Next()
	if s != nil {
		p := s.Prev()
		// Note: Cannot use multiple assignment because
		// evaluation order of LHS is not specified.
		r.next = s
		s.prev = r
		n.prev = p
		p.next = n
	}
	return n
}

// Unlink 从环 r 中移除从 r.Next() 开始算起的 n % r.Len() 个元素。
// 如果 n % r.Len() == 0，r 会保持不变。
// 返回值是移除元素后的子环。 r 必须非空。
func (r *Ring) Unlink(n int) *Ring {
	if n <= 0 {
		return nil
	}
	return r.Link(r.Move(n + 1))
}

// Len 计算环中元素的数量。
// 它按与元素数量成比例的时间执行。
// 
//
func (r *Ring) Len() int {
	n := 0
	if r != nil {
		n = 1
		for p := r.Next(); p != r; p = p.next {
			n++
		}
	}
	return n
}

// Do 对环中每一个元素按顺序调用函数 f。
// 如果 f 改变 *r，Do 的行为是不确定的。
func (r *Ring) Do(f func(interface{})) {
	if r != nil {
		f(r.Value)
		for p := r.Next(); p != r; p = p.next {
			f(p.Value)
		}
	}
}
