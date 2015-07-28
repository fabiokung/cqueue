package cqueue

import (
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/docker/docker/pkg/reexec"
	"github.com/pborman/uuid"
)

func TestMain(m *testing.M) {
	reexec.Register("enqueue", reexecEnqueue)
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

func enqueue(queue Queue, from, to uint16, randomDelays bool) error {
	n := rand.Intn(100)
	for i := from; i < to; i++ {
		if randomDelays && i%uint16(n) == 0 {
			time.Sleep(time.Duration(n) * time.Nanosecond)
		}
		if err := queue.Enqueue(i); err != nil {
			return err
		}
	}
	return nil
}

func TestDequeueEmpty(t *testing.T) {
	filename := "cqueue-test-" + uuid.New()
	defer os.RemoveAll(filepath.Join("/dev/shm", filename))

	mem, err := LoadShared(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()

	var queue Queue = mem.List()
	if i, err := queue.Dequeue(); err != ErrEmpty {
		t.Fatalf("Expected error %s to be %q. Value: %d", err, ErrEmpty, i)
	}
}

func TestEnqueueMultiple(t *testing.T) {
	filename := "cqueue-test-" + uuid.New()
	defer os.RemoveAll(filepath.Join("/dev/shm", filename))

	var (
		n uint16 = 65532
		i uint16
	)
	mem, err := LoadShared(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()

	var list List = mem.List()
	if err := enqueue(list, 0, n, false); err != nil {
		t.Fatal(err)
	}

	i = 0
	list.Walk(func(v uint16) {
		if v != i {
			t.Fatalf("Expected: %d, got: %d", i, v)
		}
		i++
	})

	for i = 0; i < n; i++ {
		v, err := list.Dequeue()
		if err != nil {
			t.Fatal(err)
		}
		if v != i {
			t.Fatalf("Expected: %d, got: %d", i, v)
		}
	}

	// one more
	if err := list.Enqueue(123); err != nil {
		t.Fatal(err)
	}
	v, err := list.Dequeue()
	if err != nil {
		t.Fatal(err)
	}
	if v != 123 {
		t.Fatalf("Expected: 123, got: %d", i, v)
	}
}

func TestParallelAccessByMultipleProcesses(t *testing.T) {
	filename := "cqueue-test-" + uuid.New()
	defer os.RemoveAll(filepath.Join("/dev/shm", filename))

	mem, err := LoadShared(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()

	// spawn 5 producers
	runEnqueue(t, filename, "0", "9999")
	runEnqueue(t, filename, "10000", "19999")
	runEnqueue(t, filename, "20000", "29999")
	runEnqueue(t, filename, "30000", "39999")
	runEnqueue(t, filename, "40000", "49999")

	var list List = mem.List()
	for dequeued := 0; dequeued < 50000; {
		if v, err := list.Dequeue(); err != nil {
			continue // no items yet
		} else {
			t.Logf("[%d] dequeued: %d", dequeued, v)
			dequeued++
		}
	}

}

func runEnqueue(t *testing.T, filename, from, to string) {
	cmd := reexec.Command("enqueue", filename, from, to)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}()

}

func reexecEnqueue() {
	filename := os.Args[1]
	mem, err := LoadShared(filename)
	if err != nil {
		panic(err)
	}
	defer mem.Close()
	from, err := strconv.ParseUint(os.Args[2], 10, 16)
	if err != nil {
		panic(err)
	}
	to, err := strconv.ParseUint(os.Args[3], 10, 16)
	if err != nil {
		panic(err)
	}
	if err := enqueue(
		mem.List(), uint16(from), uint16(to)+1, true,
	); err != nil {
		panic(err)
	}
}
