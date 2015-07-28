package cqueue

import (
	"sync/atomic"
	"unsafe"
)

const MaxLinkedListItems = 65534

type lockFreeLinkedList struct {
	head     nodePointer
	tail     nodePointer
	freeHead nodePointer
	freeTail nodePointer
	nodes    [MaxLinkedListItems]node
}

type node struct {
	value    uint16
	next     nodePointer
	nextFree nodePointer

	// immutable position in memory
	idx uint16
}

type nodePointer struct {
	index int32
	count uint32
}

func (p *nodePointer) equals(other nodePointer) bool {
	return p.index == other.index && p.count == p.count
}

func (p *nodePointer) compareAndSwap(old, other nodePointer) bool {
	addr := (*uint64)(unsafe.Pointer(p))
	oldAsInt := (*uint64)(unsafe.Pointer(&old))
	newAsInt := (*uint64)(unsafe.Pointer(&other))
	return atomic.CompareAndSwapUint64(addr, *oldAsInt, *newAsInt)
}

func (l *lockFreeLinkedList) Enqueue(v uint16) error {
	node, err := l.dequeueFree()
	if err != nil {
		return err
	}
	node.value = v
	node.next.index = -1
	node.next.count = 0

	var tail nodePointer
	for {
		tail = l.tail
		next := l.nodes[l.tail.index].next
		if l.tail.equals(tail) { // are tail and next consistent?
			if next.index < 0 { // was tail pointing to the last node?
				// try to link node at the tail
				if l.nodes[l.tail.index].next.compareAndSwap(
					next, nodePointer{
						index: int32(node.idx),
						count: next.count + 1,
					},
				) {
					break // enqueue done
				}
			} else { // tail was not pointing to the last node
				// try to swing tail to the next node
				l.tail.compareAndSwap(tail, nodePointer{
					index: next.index,
					count: tail.count + 1,
				})
			}
		}
	}

	// Enqueue is done, try to swing tail to the new node
	l.tail.compareAndSwap(tail, nodePointer{
		index: int32(node.idx),
		count: tail.count + 1,
	})
	return nil
}

func (l *lockFreeLinkedList) Dequeue() (uint16, error) {
	var (
		value uint16
		head  nodePointer
	)
	for {
		head = l.head
		tail := l.tail
		next := l.nodes[head.index].next
		if l.head.equals(head) { // are head, tail, and next consistent?
			if head.index == tail.index { // is queue empty or tail falling behind?
				if next.index == -1 { // is queue empty?
					return 0, ErrEmpty
				}
				// tail is falling behind. Try to advance it
				l.tail.compareAndSwap(tail, nodePointer{
					index: next.index,
					count: tail.count + 1,
				})
			} else {
				// read before compareAndSwap, otherwise another
				// dequeue might free the next node
				value = l.nodes[next.index].value
				// try to swing head to the next node
				if l.head.compareAndSwap(head, nodePointer{
					index: next.index,
					count: head.count + 1,
				}) {
					break // dequeue is done
				}
			}
		}
	}
	// it is safe now to free the old node
	if err := l.enqueueFree(&l.nodes[head.index]); err != nil {
		return 0, err
	}
	return value, nil
}

func (l *lockFreeLinkedList) Walk(fn NodeFunc) {
	for i := l.head.index; i > 0; i = l.nodes[i].next.index {
		fn(l.nodes[i].value)
	}
}
func (l *lockFreeLinkedList) Empty() bool {
	return l.head.index == l.tail.index
}

func (l *lockFreeLinkedList) enqueueFree(node *node) error {
	// reset node
	node.value = 0
	node.next.index = -1
	node.next.count = 0
	node.nextFree.index = -1
	node.nextFree.count = 0

	var tail nodePointer
	// similar to Enqueue, but on the free (linked)list
	for {
		tail = l.freeTail
		next := l.nodes[tail.index].nextFree
		if l.freeTail.equals(tail) {
			if next.index == -1 {
				if l.nodes[tail.index].nextFree.compareAndSwap(
					next, nodePointer{
						index: int32(node.idx),
						count: next.count + 1,
					},
				) {
					break
				}
			} else {
				l.freeTail.compareAndSwap(tail, nodePointer{
					index: next.index,
					count: tail.count + 1,
				})
			}
		}
	}
	l.freeTail.compareAndSwap(tail, nodePointer{
		index: int32(node.idx),
		count: tail.count + 1,
	})
	return nil
}

func (l *lockFreeLinkedList) dequeueFree() (*node, error) {
	var (
		free *node
		head nodePointer
	)
	// similar to Dequeue, but on the free (linked)list
	for {
		head = l.freeHead
		tail := l.freeTail
		next := l.nodes[head.index].nextFree
		if l.freeHead.equals(head) {
			if head.index == tail.index {
				if next.index == -1 {
					return nil, ErrFreeListEmpty
				}
				l.freeTail.compareAndSwap(tail, nodePointer{
					index: next.index,
					count: tail.count + 1,
				})
			} else {
				free = &l.nodes[next.index]
				if l.freeHead.compareAndSwap(head, nodePointer{
					index: next.index,
					count: head.count + 1,
				}) {
					break
				}
			}
		}
	}
	l.nodes[head.index].nextFree.index = -1
	l.nodes[head.index].nextFree.count = 0
	return free, nil
}
