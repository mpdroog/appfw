// github.com/leprosus/golang-ttl-map v1.1.7
package ttl_map

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

// Spawn 100 go-routines and try to concurrently read/write the same key
func TestConcurrentSameKey(t *testing.T) {
	os.Remove("./TestConcurrentSameKey.tsv")
	t.Parallel()

	wg := new(sync.WaitGroup)
	heap := New("./TestConcurrentSameKey.tsv", 1024)
	if e := heap.Load(); e != nil {
		t.Fatal(e)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			if n == 70 {
				// Save
				if e := heap.Save(); e != nil {
					t.Error(e)
				}

			} else {
				// Add
				num := heap.GetInt("TestConcurrent")
				num++
				if e := heap.Set("TestConcurrent", num, 100, LIMIT_MAX); e != nil {
					t.Error(e)
				}
			}
		}(i)
	}

	wg.Wait()

	heap = New("./TestConcurrentSameKey.tsv", 10)
	if e := heap.Load(); e != nil {
		t.Fatal(e)
	}
}

// Spawn 100 go-routines and try to concurrently read/write random keys
func TestConcurrentDifferentKey(t *testing.T) {
	os.Remove("./TestConcurrentDifferentKey.tsv")
	t.Parallel()

	wg := new(sync.WaitGroup)
	heap := New("./TestConcurrentDifferentKey.tsv", 1024)
	if e := heap.Load(); e != nil {
		t.Fatal(e)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			if n == 70 {
				// Save
				if e := heap.Save(); e != nil {
					t.Error(e)
				}

			} else {
				// Add
				num := heap.GetInt(fmt.Sprintf("TestConcurrent-%d", n))
				num++
				if e := heap.Set(fmt.Sprintf("TestConcurrent-%d", n), num, 10, LIMIT_MAX); e != nil {
					t.Error(e)
				}
			}
		}(i)
	}

	wg.Wait()

	heap = New("./TestConcurrentDifferentKey.tsv", 10)
	if e := heap.Load(); e != nil {
		t.Fatal(e)
	}

	// ensure key exists
	for n := 0; n < 100; n++ {
		if n == 70 {
			// 70 = save in-between
			continue
		}
		num := heap.GetInt(fmt.Sprintf("TestConcurrent-%d", n))
		if num != 1 {
			t.Errorf("Missing key=%d", n)
		}
	}
}
