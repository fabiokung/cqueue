package cqueue

import (
	"log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var storageSize int // bytes

func init() {
	l := &lockFreeLinkedList{}
	mem := sharedMemory{
		list: l,
		raw:  make([]byte, 0, 0),
	}
	storageSize = int(unsafe.Sizeof(mem)) +
		int(unsafe.Sizeof(mem.raw)) +
		int(unsafe.Sizeof(l)) +
		int(unsafe.Sizeof(l.head)) +
		int(unsafe.Sizeof(l.tail)) +
		int(unsafe.Sizeof(l.freeHead)) +
		int(unsafe.Sizeof(l.freeTail)) +
		int(unsafe.Sizeof(l.nodes)) +
		4*1024 // safety padding
}

type sharedMemory struct {
	list List
	raw  []byte
}

func LoadShared(name string) (Memory, error) {
	// TODO: portable shm_open
	filename := filepath.Join("/dev/shm", name)
	if file, err := os.OpenFile(
		filename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600,
	); err == nil {
		// new file
		defer file.Close()

		log.Printf(
			"Creating a shared memory object (%s) of size: %d bytes",
			filename, storageSize,
		)
		// TODO: prevent other processes from reading until setup is done
		if err := syscall.Ftruncate(
			int(file.Fd()), int64(storageSize),
		); err != nil {
			return nil, err
		}
		return initMemory(file)
	}

	file, err := os.OpenFile(filename, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return loadMemory(file)
}

func (m *sharedMemory) List() List {
	return m.list
}

func (m *sharedMemory) Close() error {
	m.list = nil
	return syscall.Munmap(m.raw)
}

type sliceType struct {
	data unsafe.Pointer
	len  int
	cap  int
}

func initMemory(f *os.File) (*sharedMemory, error) {
	data, err := syscall.Mmap(int(f.Fd()), 0, storageSize,
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED,
	)
	if err != nil {
		return nil, err
	}
	slice := (*sliceType)(unsafe.Pointer(&data))
	list := (*lockFreeLinkedList)(slice.data)

	// link all nodes in the free list
	for i := range list.nodes {
		list.nodes[i].value = 0
		list.nodes[i].idx = uint16(i)
		list.nodes[i].next.index = -1
		list.nodes[i].next.count = 0
		list.nodes[i].nextFree.index = int32(i) + 1
		list.nodes[i].nextFree.count = 0
	}
	last := int32(cap(list.nodes) - 1)
	list.nodes[last].nextFree.index = -1 // last points to NULL

	// allocate 1 node to start
	list.head.index = 0
	list.head.count = 0
	list.tail.index = 0
	list.tail.count = 0
	list.nodes[0].next.index = -1
	list.nodes[0].nextFree.index = -1

	// all other nodes are free
	list.freeHead.index = 1
	list.freeHead.count = 0
	list.freeTail.index = last
	list.freeTail.count = 0

	return &sharedMemory{list: list, raw: data}, nil
}

func loadMemory(f *os.File) (*sharedMemory, error) {
	data, err := syscall.Mmap(int(f.Fd()), 0, storageSize,
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED,
	)
	if err != nil {
		return nil, err
	}
	slice := (*sliceType)(unsafe.Pointer(&data))
	return &sharedMemory{
		list: (*lockFreeLinkedList)(slice.data),
		raw:  data,
	}, nil
}
