// github.com/leprosus/golang-ttl-map v1.1.7
package ttl_map

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Data struct {
	Key       string
	Value     interface{}
	Max       int
	Timestamp int64
}

type Heap struct {
	//TODO: replace all mutex stuff with sync.Map?
	dataMx *sync.RWMutex
	fileMx *sync.Mutex
	wg     *sync.WaitGroup

	data map[string]Data

	filePath   string
	withSaving uint32
	queue      chan Data

	errFn     func(err error)
	errFnInit bool

	closed bool
}

func New() *Heap {
	return &Heap{
		dataMx:     &sync.RWMutex{},
		wg:         &sync.WaitGroup{},
		withSaving: 0,

		data: map[string]Data{},
	}
}

func (h *Heap) Path(filePath string) {
	withSaving := atomic.LoadUint32(&h.withSaving)

	if withSaving == 0 {
		h.filePath = filePath
		h.queue = make(chan Data, 1024)
		h.fileMx = &sync.Mutex{}

		go h.handle()

		atomic.StoreUint32(&h.withSaving, 1)
	} else {
		h.fileMx.Lock()
		h.filePath = filePath
		h.fileMx.Unlock()
	}
}

func (h *Heap) handle() {
	var err error
	for data := range h.queue {
		if h.closed {
			return
		}

		err = h.append(data)
		h.wg.Done()
		if err != nil && h.errFnInit {
			h.errFn(err)
		}
	}
}

func (h *Heap) append(data Data) (err error) {
	h.fileMx.Lock()
	defer h.fileMx.Unlock()

	if h.closed {
		return
	}

	var file *os.File
	file, err = os.OpenFile(h.filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		return
	}
	defer func() {
		_ = file.Sync()
	}()
	defer func() {
		_ = file.Close()
	}()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	err = enc.Encode(data)
	if err != nil {
		return
	}

	bs := buf.Bytes()
	bs = append(bs, '\n')

	_, err = file.Write(bs)
	if err != nil {
		return
	}

	return
}

func (h *Heap) Error(fn func(err error)) {
	h.errFn = fn
	h.errFnInit = true
}

func (h *Heap) Set(key string, value interface{}, ttl int64, max int) {
	if ttl == 0 {
		return
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

	h.dataMx.Lock()
	if h.closed {
		return
	}
	h.data[key] = data
	h.dataMx.Unlock()

	data.Key = key

	withSaving := atomic.LoadUint32(&h.withSaving)
	if withSaving > 0 {
		h.wg.Add(1)
		h.queue <- data
	}
}

// SetValue overwrites value but only sets TTL once
func (h *Heap) SetValue(key string, value interface{}, ttl int64, max int) {
	if ttl == 0 {
		panic("DevErr, ttl is 0 for Update")
	}

	var (
		data Data
		ok   bool
	)
	h.dataMx.RLock()
	if h.closed {
		return
	}
	data, ok = h.data[key]
	h.dataMx.RUnlock()

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

	h.dataMx.Lock()
	h.data[key] = data
	h.dataMx.Unlock()

	data.Key = key

	withSaving := atomic.LoadUint32(&h.withSaving)
	if withSaving > 0 {
		h.wg.Add(1)
		h.queue <- data
	}
}

func (h *Heap) Get(key string) (val interface{}, ok bool) {
	var data Data
	h.dataMx.RLock()
	data, ok = h.data[key]
	h.dataMx.RUnlock()

	if ok {
		if data.Timestamp != -1 && data.Timestamp <= time.Now().Unix() {
			h.Del(key)

			ok = false
		} else {
			val = data.Value
		}
	}

	return
}

func (h *Heap) Len() (l int) {
	h.dataMx.RLock()
	l = len(h.data)
	h.dataMx.RUnlock()

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

func (h *Heap) Del(key string) {
	h.dataMx.RLock()
	if h.closed {
		return
	}
	_, ok := h.data[key]
	h.dataMx.RUnlock()
	if !ok {
		return
	}

	h.dataMx.Lock()
	delete(h.data, key)
	h.dataMx.Unlock()

	withSaving := atomic.LoadUint32(&h.withSaving)
	if withSaving > 0 {
		h.wg.Add(1)
		h.queue <- Data{
			Key:       key,
			Timestamp: 0,
		}
	}
}

func (h *Heap) Range(fn func(key string, value interface{}, ttl int64, max int)) {
	data := map[string]Data{}

	h.dataMx.Lock()
	for key, val := range h.data {
		data[key] = val
	}
	h.dataMx.Unlock()

	for _, d := range data {
		fn(d.Key, d.Value, d.Timestamp, d.Max)
	}
}

func (h *Heap) Support(kind interface{}) {
	gob.Register(kind)
}

func (h *Heap) Fork() (data map[string]Data) {
	data = make(map[string]Data)

	h.dataMx.Lock()
	for key, val := range h.data {
		data[key] = val
	}
	h.dataMx.Unlock()

	return
}

func (h *Heap) Close() {
	h.fileMx.Lock()
	defer h.fileMx.Unlock()

	h.closed = true
}

func (h *Heap) Save() (err error) {
	withSaving := atomic.LoadUint32(&h.withSaving)
	if withSaving == 0 {
		return
	}

	h.fileMx.Lock()
	defer h.fileMx.Unlock()

	h.wg.Add(1)
	defer h.wg.Done()

	var file *os.File
	file, err = os.OpenFile(h.filePath+".sav", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return
	}
	defer func() {
		if e := file.Close(); e != nil {
			fmt.Printf("WARN: Skipped error e=%s\n", e.Error())
		}
	}()

	var (
		bs  []byte
		buf bytes.Buffer
	)

	h.dataMx.RLock()
	defer h.dataMx.RUnlock()

	for key, data := range h.data {
		if data.Timestamp != -1 && data.Timestamp < time.Now().Unix() {
			// Also cleaning up memory here
			delete(h.data, key)
			continue
		}

		buf.Reset()
		enc := gob.NewEncoder(&buf)
		err = enc.Encode(data)
		if err != nil {
			return
		}

		bs = buf.Bytes()
		bs = append(bs, '\n')

		_, err = file.Write(bs)
		if err != nil {
			return
		}
	}

	// Only remove file when it exists
	if _, err = os.Stat(h.filePath); err == nil {
		if err = os.Remove(h.filePath); err != nil {
			return
		}
	}

	err = os.Rename(h.filePath+".sav", h.filePath)
	return
}

func (h *Heap) Restore() (err error) {
	withSaving := atomic.LoadUint32(&h.withSaving)
	if withSaving == 0 {
		return
	}

	h.fileMx.Lock()
	defer h.fileMx.Unlock()

	_, err = os.Stat(h.filePath)
	if err != nil {
		return
	}

	var file *os.File
	file, err = os.OpenFile(h.filePath, os.O_RDONLY, 0755)
	if err != nil {
		return
	}
	defer func() {
		if e := file.Sync(); e != nil {
			fmt.Printf("WARN: file.Sync e=%s\n", e.Error())
		}
		if e := file.Close(); e != nil {
			fmt.Printf("WARN: file.Close e=%s\n", e.Error())
		}
	}()

	reader := bufio.NewReader(file)

	var (
		bs   []byte
		buf  bytes.Buffer
		data Data
		heap = map[string]Data{}
		now  = time.Now().Unix()
	)

	for {
		bs, err = reader.ReadBytes('\n')
		if err == io.EOF {
			err = nil

			break
		}

		if err != nil {
			return
		}

		buf.Reset()
		dec := gob.NewDecoder(&buf)

		bs = bs[:len(bs)-1]
		buf.Write(bs)

		err = dec.Decode(&data)
		if err != nil {
			return
		}

		if data.Timestamp > -1 && data.Timestamp < now {
			continue
		}

		heap[data.Key] = data
	}

	h.dataMx.Lock()
	h.data = heap
	h.dataMx.Unlock()

	return
}

func (h *Heap) Wait() {
	h.wg.Wait()
}
