## Concurrent queues

Lock-free (non-blocking) concurrent queue implementation on top of shared memory
that supports multiple processes as producers and consumers, based on the ideas
described at ["Simple, fast, and practical non-blocking and blocking concurrent
queue algorithms"][paper]:

> Maged M. Michael and Michael L. Scott. 1996. "Simple, fast, and practical
> non-blocking and blocking concurrent queue algorithms". *In Proceedings of the
> fifteenth annual ACM symposium on Principles of distributed computing
> (PODC '96). ACM*, New York, NY, USA, 267-275. DOI=10.1145/248052.248106
> http://doi.acm.org/10.1145/248052.248106

The algorithm proposed was modified to work properly on shared regions of
memory. In particular, memory allocation needs to manage a fixed region of
memory, which was implemented as a free-list of nodes, embedded as another
linked list and sharing nodes with the main data structure.


### Usage

Multiple processes can safely enqueue and dequeue items from the same shared
region, as long as they use the same name (`shared-region` in the example
below):

```go
package main

import(
	"log"
	"github.com/fabiokung/cqueue"
)

func main() {
	mem, err := cqueue.LoadShared("shared-region")
	if err != nil {
		panic(err)
	}
	defer mem.Close()

	queue := mem.List()
	if err := queue.Enqueue(123); err != nil {
		panic(err)
	}
	v, err := queue.Dequeue()
	if err != nil {
		panic(err)
	}
	log.Println(v) // 123
}
```

See tests for more examples and usage, including multiple processes sharing the
same queue. You can also try running it yourself with `go test`.

### Known limitations

* Max of `65534` items, because I need tight control over the memory layout and
  I did not have time to make it very dynamic yet.

* Linux only, I have not had time to make shm usage portable (with `shm_open(3)`
  on darwin/osx, for example).

* Queue state is on `tmpfs`, but nothing prevents it to be on durable
  filesystems/storage, as long as `mmap` semantics are preserved. A durable
  storage plus an append only journal could make this an interesting option for
  a persistent/durable concurrent queue.

* Only `uint16` values can be queued for now.

* Processes crashing right after successfully Dequeueing an item, but before a
  node is added back to the freelist, can cause memory to leak (nodes can get
  orphaned). This can possibly be solved with a stop-the-world reaper that
  detects orphaned nodes and add them back to the freelist. Maybe someday.

* Concurrent Queues can currently be safely shared across multiple processes,
  but it may be lacking some memory barriers to allow thread safety inside a
  single process (across many goroutines). The Go runtime guarantees ordering
  without explicit synchronization for a single goroutine. This shouldn't be too
  hard to fix.

[paper]: http://dl.acm.org/citation.cfm?id=248106
