// Inspired by: github.com/leprosus/golang-ttl-map v1.1.7
package ttl_map

import (
	"fmt"
	"sync"
	"time"
)

type Data struct {
	Key       string
	Value     int
	Max       int
	Timestamp int64
}

type Heap struct {
	fileMx *sync.RWMutex
	data   *sync.Map

	filePath string
	queue    chan Data
	errFn    func(err error)
	closed   bool
}

// Get new instance and spawn a go-routine for updates
func New(filePath string, queueSize int) (h *Heap) {
	h = &Heap{
		fileMx:   new(sync.RWMutex),
		filePath: filePath,
		queue:    make(chan Data, queueSize),
		data:     new(sync.Map),
	}
	return
}

func (h *Heap) Set(key string, value int, ttl int64, max int) error {
	if ttl == 0 {
		panic("Invalid TTL-value")
	}
	if h.closed {
		return fmt.Errorf("Closed")
	}

	data := Data{
		Key:       key,
		Value:     value,
		Max:       max,
		Timestamp: time.Now().Unix(),
	}

	if ttl > 0 {
		data.Timestamp += ttl
	} else if ttl < 0 {
		data.Timestamp = -1
	}

	if cap(h.queue) == 0 {
		return fmt.Errorf("Queue full")
	}

	h.queue <- data
	h.data.Store(key, data)
	return nil
}

// SetValue overwrites value but only sets TTL once
func (h *Heap) SetValue(key string, value int, ttl int64, max int) error {
	if ttl == 0 {
		panic("DevErr, ttl is 0 for Update")
	}
	if h.closed {
		return fmt.Errorf("Closed")
	}

	var (
		data Data
	)
	bin, ok := h.data.Load(key)
	if ok {
		data = bin.(Data)
	}

	if ok {
		// Update value
		data.Key = key
		data.Value = value
		data.Max = max
	} else {
		// Add value with TTL
		data = Data{
			Key:       key,
			Value:     value,
			Max:       max,
			Timestamp: time.Now().Unix(),
		}

		if ttl > 0 {
			data.Timestamp += ttl
		} else if ttl < 0 {
			data.Timestamp = -1
		}
	}

	if cap(h.queue) == 0 {
		return fmt.Errorf("Queue full")
	}
	h.queue <- data
	h.data.Store(key, data)
	return nil
}

func (h *Heap) Get(key string) (val interface{}, ok bool) {
	var data Data
	bin, ok := h.data.Load(key)
	if ok {
		data = bin.(Data)
	}

	if ok {
		if data.Timestamp <= time.Now().Unix() {
			if e := h.Del(key); e != nil {
				fmt.Printf("WARN: heap.Del e=%s\n", e.Error())
			}

			ok = false
		} else {
			val = data.Value
		}
	}

	return
}

func (h *Heap) Len() (l int) {
	h.data.Range(func(key, value interface{}) bool {
		l++
		return true
	})
	return
}

// GetInt assumes the type and directly returns it
func (h *Heap) GetInt(key string) int {
	val, ok := h.Get(key)
	if !ok {
		return 0
	}
	return val.(int)
}

func (h *Heap) Del(key string) error {
	if h.closed {
		return fmt.Errorf("Closed")
	}
	if cap(h.queue) == 0 {
		// TODO: Correct?
		return fmt.Errorf("Queue full")
	}

	_, ok := h.data.LoadAndDelete(key)
	if !ok {
		return nil
	}
	h.queue <- Data{
		Key:       key,
		Timestamp: 0,
	}
	return nil
}

func (h *Heap) Range(fn func(key string, value interface{}, ttl int64, max int)) {
	h.data.Range(func(key, value interface{}) bool {
		d := value.(Data)
		fn(d.Key, d.Value, d.Timestamp, d.Max)
		return true
	})
}

func (h *Heap) Fork() (data map[string]Data) {
	data = make(map[string]Data)

	h.data.Range(func(key, value interface{}) bool {
		data[key.(string)] = value.(Data)
		return true
	})

	return
}
