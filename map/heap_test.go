// github.com/leprosus/golang-ttl-map v1.1.7
package ttl_map

import (
	"fmt"
	"testing"
	"time"
)

func TestTTL(t *testing.T) {
	heap := New("./TestTTL.tsv", 10)
	{
		num := heap.GetInt("test_ttl")
		if num != 0 {
			t.Errorf("test_ttl not 0 but %d", num)
		}
		num++
		if e := heap.Set("test_ttl", num, 1, 10); e != nil {
			t.Error(e)
		}
	}

	{
		num := heap.GetInt("test_ttl")
		if num != 1 {
			v, _ := heap.Get("test_ttl")
			fmt.Printf("%+v\n", v)
			t.Errorf("test_ttl not 1 but %d", num)
		}
	}

	// ensure entry is 'expired'
	time.Sleep(time.Second * 1)

	{
		num := heap.GetInt("test_ttl")
		if num != 0 {
			t.Errorf("test_ttl not 0 but %d", num)
		}
	}

	heap.Range(func(key string, value interface{}, ttl int64, max int) {
		if testing.Verbose() {
			fmt.Printf("TestTTL heap.Range k=%s v=%s\n", key, value)
		}
		t.Errorf("heap.Range not empty as expected found key=%s", key)
	})
}

func TestTTLSetValue(t *testing.T) {
	heap := New("./TestTTLSetValue.tsv", 10)
	{
		num := heap.GetInt("test_add")
		if num != 0 {
			t.Errorf("test_add not 0 but %d", num)
		}
		num++
		if e := heap.Set("test_add", num, 1, 10); e != nil {
			t.Error(e)
		}
	}

	// ensure entry is 'expired'
	time.Sleep(time.Second * 1)

	{
		num := heap.GetInt("test_add")
		if num != 0 {
			t.Errorf("test_add not 0 but %d", num)
		}
		num = num + 2

		if e := heap.SetValue("test_add", num, 1, 10); e != nil {
			t.Error(e)
		}
	}

	{
		num := heap.GetInt("test_add")
		if num != 2 {
			t.Errorf("test_add not 2 but %d", num)
		}
	}
}
