package ttl_map

import (
	"bytes"
	//"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
)

func (h *Heap) handle() {
	var err error

	for data := range h.queue {
		if h.closed {
			return
		}
		err = h.append(data)
		if err != nil {
			fmt.Printf("WARN heap.handle e=%s\n", err.Error())
		}
	}
}

// Internal go-routine to append to file
func (h *Heap) append(data Data) (err error) {
	if h.closed {
		fmt.Printf("WARN: Lost %v\n", data)
		return
	}

	h.fileMx.Lock()
	defer h.fileMx.Unlock()

	var file *os.File
	file, err = os.OpenFile(h.filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		return
	}

	defer func() {
		// TODO: errfn?
		if e := file.Sync(); e != nil {
			fmt.Printf("WARN: heap(file.Sync) e=%s\n", e.Error())
		}
		if e := file.Close(); e != nil {
			fmt.Printf("WARN: heap(file.Close) e=%s\n", e.Error())
		}
	}()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	err = enc.Encode(data)
	if err != nil {
		return
	}

	_, err = file.Write(buf.Bytes())
	if err != nil {
		return
	}

	return
}
