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

[paper]: http://dl.acm.org/citation.cfm?id=248106
