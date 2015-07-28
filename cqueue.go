package cqueue

import "errors"

var (
	ErrEmpty         = errors.New("List empty")
	ErrFreeListEmpty = errors.New("List full")
)

type NodeFunc func(uint16)

type Queue interface {
	Enqueue(v uint16) error
	Dequeue() (uint16, error)
	Empty() bool
}

type List interface {
	Queue
	Walk(NodeFunc)
}

type Memory interface {
	List() List
	Close() error
}
