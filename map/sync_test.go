package ttl_map

import (
	"os"
	"testing"
)

// Check if file not exists doesn't give any error
func TestLoadNoFile(t *testing.T) {
	heap := New("./TestLoadNoFile.tsv", 10)

	// TODO: Variable?
	if e := heap.Load(); e != nil {
		t.Fatal(e)
	}
}

// Add entry, save, remove, load and check if there
func TestStoreLoad(t *testing.T) {
	os.Remove("./TestStoreLoad.tsv")
	{
		heap := New("./TestStoreLoad.tsv", 10)

		if e := heap.Set("Test1", 1, 100, 1); e != nil {
			t.Error(e)
		}
		if e := heap.Set("Test2", 1, 100, 1); e != nil {
			t.Error(e)
		}

		if e := heap.Save(); e != nil {
			t.Fatal(e)
		}
		heap.Close()
	}

	{
		heap := New("./TestStoreLoad.tsv", 10)

		// Silly action to ensure we don't break anything
		{
			if e := heap.Del("Test1"); e != nil {
				t.Error(e)
			}
			if e := heap.Del("Test2"); e != nil {
				t.Error(e)
			}
		}

		if e := heap.Load(); e != nil {
			t.Fatal(e)
		}

		_, ok := heap.Get("Test1")
		if !ok {
			t.Errorf("Key Test1 not found")
		}
		_, ok = heap.Get("Test2")
		if !ok {
			t.Errorf("Key Test2 not found")
		}
	}
}
