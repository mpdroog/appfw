package ttl_map

import (
	"bufio"
	"bytes"
	"time"
	//"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

func checkFileExists(filePath string) (ok bool, err error) {
	ok = true
	_, err = os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		ok = false
		err = nil
	}
	return ok, err
}

func (h *Heap) Load() (err error) {
	if h.closed {
		return fmt.Errorf("Closed")
	}

	h.fileMx.Lock()
	defer h.fileMx.Unlock()

	var file *os.File
	file, err = os.OpenFile(h.filePath, os.O_RDONLY|os.O_CREATE, 0755)
	if err != nil {
		return
	}
	defer func() {
		if e := file.Close(); e != nil {
			fmt.Printf("WARN: queue.Load(Close) e=%s\n", e.Error())
		}
	}()

	dec := json.NewDecoder(bufio.NewReader(file))
	now := time.Now().Unix()
	for {
		var v Data
		err = dec.Decode(&v)
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}
		if v.Timestamp < now {
			// expired, don't load
			continue
		}
		h.data.Store(v.Key, v)
	}

	go h.handle()
	return
}

func (h *Heap) Save() (err error) {
	if h.closed {
		return fmt.Errorf("Closed")
	}

	h.fileMx.Lock()
	defer h.fileMx.Unlock()

	var file *os.File
	file, err = os.OpenFile(h.filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return
	}
	defer func() {
		if e := file.Sync(); e != nil {
			fmt.Printf("WARN: queue.Backup(Sync) e=%s\n", e.Error())
		}
		if e := file.Close(); e != nil {
			fmt.Printf("WARN: queue.Backup(Close) e=%s\n", e.Error())
		}
	}()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	now := time.Now().Unix()

	h.data.Range(func(key, value interface{}) bool {
		v := value.(Data)
		if v.Timestamp < now {
			// expired, don't save
			return true
		}

		if err = enc.Encode(value); err != nil {
			return false
		}
		/*if _, err = buf.WriteString("\n"); err != nil {
			return false
		}*/
		return true
	})
	if err != nil {
		return
	}

	_, err = file.Write(buf.Bytes())
	return
}

func (h *Heap) Close() error {
	h.closed = true
	return nil
}

// Delete is a helper func to remove the state-file (if it exists)
func (h *Heap) Delete() (e error) {
	ok := false

	ok, e = checkFileExists(h.filePath)
	if e != nil {
		return
	}
	if !ok {
		// No such file, ignore
		return
	}

	e = os.Remove(h.filePath)
	return
}
