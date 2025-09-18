package util

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestConcurrentQueuePushPop(t *testing.T) {
	q := NewConcurrentQueue[int](5)
	defer q.Close()

	err := q.Push(42)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	item, err := q.Pop()
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}
	if item != 42 {
		t.Errorf("expected 42, got %d", item)
	}
}

func TestConcurrentQueueFullQueue(t *testing.T) {
	bufSize := 3
	q := NewConcurrentQueue[int](bufSize)
	defer q.Close()

	for i := range bufSize {
		err := q.Push(i)
		if err != nil {
			t.Fatalf("Push failed for item %d: %v", i, err)
		}
	}

	// Try to push one more item - should fail
	err := q.Push(999)
	if err == nil {
		t.Fatal("expected error when queue is full")
	}
	if !errors.Is(err, QueueIsFullError) {
		t.Errorf("expected QueueIsFullError, got %v", err)
	}
}

func TestConcurrentQueueFIFO(t *testing.T) {
	q := NewConcurrentQueue[int](10)
	defer q.Close()

	// Push items
	for i := range 5 {
		err := q.Push(i)
		if err != nil {
			t.Fatalf("Push failed for item %d: %v", i, err)
		}
	}

	// Pop items and verify FIFO order
	for i := range 5 {
		item, err := q.Pop()
		if err != nil {
			t.Fatalf("Pop failed: %v", err)
		}
		if item != i {
			t.Errorf("expected %d, got %d", i, item)
		}
	}
}

func TestConcurrentQueueClose(t *testing.T) {
	q := NewConcurrentQueue[int](10)

	// Push some items
	q.Push(1)
	q.Push(2)

	// Close the queue
	q.Close()

	// Try to push after close - should fail
	err := q.Push(3)
	if err == nil {
		t.Fatal("expected error when pushing to closed queue")
	}
	if !errors.Is(err, QueueIsClosedError) {
		t.Errorf("expected QueueIsClosedError, got %v", err)
	}

	// Try to pop after close - should fail
	_, err = q.Pop()
	if err == nil {
		t.Fatal("expected error when popping from closed queue")
	}
	if !errors.Is(err, QueueIsClosedError) {
		t.Errorf("expected QueueIsClosedError, got %v", err)
	}

	// Closing again should be safe
	q.Close()
}

func TestConcurrentQueueConcurrentAccess(t *testing.T) {
	q := NewConcurrentQueue[int](100)
	defer q.Close()

	var wg sync.WaitGroup
	numProducers := 5
	numConsumers := 3
	itemsPerProducer := 20

	// Start producers
	for p := range numProducers {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			for i := range itemsPerProducer {
				item := producerID*1000 + i
				for {
					err := q.Push(item)
					if err == nil {
						break
					}
					if errors.Is(err, QueueIsFullError) {
						time.Sleep(time.Millisecond)
						continue
					}
					t.Errorf("unexpected error from Push: %v", err)
					return
				}
			}
		}(p)
	}

	// Collect all items
	items := make(chan int, numProducers*itemsPerProducer)

	// Start consumers
	for i := range numConsumers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				item, err := q.Pop()
				if err != nil {
					if errors.Is(err, QueueIsClosedError) {
						return
					}
					t.Errorf("unexpected error from Pop: %v", err)
					return
				}
				items <- item
				if len(items) == numProducers*itemsPerProducer {
					q.Close()
					return
				}

			}
		}(i)
	}

	wg.Wait()
	close(items)

	// Verify we got all items
	receivedItems := make(map[int]bool)
	for item := range items {
		receivedItems[item] = true
	}

	expectedCount := numProducers * itemsPerProducer
	if len(receivedItems) != expectedCount {
		t.Errorf("expected %d unique items, got %d", expectedCount, len(receivedItems))
	}
}

func TestConcurrentQueueBlockingPop(t *testing.T) {
	q := NewConcurrentQueue[string](5)
	defer q.Close()

	var wg sync.WaitGroup
	received := make(chan string, 1)

	// Start a goroutine that will block on Pop
	wg.Add(1)
	go func() {
		defer wg.Done()
		item, err := q.Pop()
		if err != nil {
			t.Errorf("Pop failed: %v", err)
			return
		}
		received <- item
	}()

	// Give the goroutine time to start and block
	time.Sleep(10 * time.Millisecond)

	// Push an item to unblock the Pop
	err := q.Push("test")
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	wg.Wait()

	select {
	case item := <-received:
		if item != "test" {
			t.Errorf("expected 'test', got '%s'", item)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Pop did not unblock after Push")
	}
}

func TestConcurrentQueueCloseUnblocksPop(t *testing.T) {
	q := NewConcurrentQueue[int](5)

	var wg sync.WaitGroup
	popDone := make(chan bool, 1)

	// Start a goroutine that will block on Pop
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := q.Pop()
		if err == nil {
			t.Error("expected error from Pop after close")
		} else if !errors.Is(err, QueueIsClosedError) {
			t.Errorf("expected QueueIsClosedError, got %v", err)
		}
		popDone <- true
	}()

	// Close the queue to unblock the Pop
	q.Close()

	wg.Wait()

	select {
	case <-popDone:
		// Success - Pop was unblocked by close
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Pop was not unblocked by close")
	}
}

func TestConcurrentQueueWithDifferentTypes(t *testing.T) {
	// Test with string type
	stringQ := NewConcurrentQueue[string](3)
	defer stringQ.Close()

	stringQ.Push("hello")
	stringQ.Push("world")

	item1, _ := stringQ.Pop()
	item2, _ := stringQ.Pop()

	if item1 != "hello" || item2 != "world" {
		t.Errorf("expected 'hello' and 'world', got '%s' and '%s'", item1, item2)
	}

	// Test with struct type
	type TestStruct struct {
		ID   int
		Name string
	}

	structQ := NewConcurrentQueue[TestStruct](2)
	defer structQ.Close()

	testItem := TestStruct{ID: 1, Name: "test"}
	structQ.Push(testItem)

	receivedItem, _ := structQ.Pop()
	if receivedItem.ID != 1 || receivedItem.Name != "test" {
		t.Errorf("expected {1, 'test'}, got {%d, '%s'}", receivedItem.ID, receivedItem.Name)
	}
}
