package writer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/schema"
	prettyjson "github.com/hokaccha/go-prettyjson"
	"github.com/vmihailenco/msgpack"
)

var formDecoder = schema.NewDecoder()

// Decode function
func Decode(r *http.Request, d interface{}) error {
	ctype := r.Header.Get("Content-Type")
	if idx := strings.Index(ctype, ";"); idx > -1 {
		ctype = ctype[:idx]
	}
	ctype = strings.TrimSpace(ctype)

	if ctype == "application/json" {
		defer r.Body.Close()
		e := json.NewDecoder(r.Body).Decode(d)
		if e == io.EOF {
			e = fmt.Errorf("io.EOF: Forgot to submit body?")
		}
		return e
	} else if ctype == "application/x-www-form-urlencoded" {
		if e := r.ParseForm(); e != nil {
			return e
		}
		return formDecoder.Decode(d, r.PostForm)
	} else if ctype == "application/x-msgpack" {
		defer r.Body.Close()
		return msgpack.NewDecoder(r.Body).Decode(d)
	}

	return fmt.Errorf("Invalid Content-Type=%s", ctype)
}

// Encode function
func Encode(w http.ResponseWriter, r *http.Request, data interface{}) error {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		// Machine, dense output
		enc := json.NewEncoder(w)
		w.Header().Set("Content-Type", "application/json")
		if e := enc.Encode(data); e != nil {
			return e
		}
		return nil
	}
	if strings.Contains(accept, "application/x-msgpack") {
		s, e := msgpack.Marshal(data)
		if e != nil {
			return e
		}
		w.Header().Set("Content-Type", "application/x-msgpack")
		w.Write(s)
		return nil
	}

	isCurl := strings.Contains(r.Header.Get("User-Agent"), "curl/")
	if isCurl {
		// Coloured output for CLI
		s, e := prettyjson.Marshal(data)
		if e != nil {
			return e
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(s)
		return nil
	}

	// JSON idented
	s, e := json.MarshalIndent(data, "", "  ")
	if e != nil {
		return e
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(s)
	w.Write([]byte("\r\n"))
	return nil
}

// ErrorRes struct
type ErrorRes struct {
	Error  string
	Detail interface{}
}

// Err return a error based on the ErroRes struct format
func Err(w http.ResponseWriter, r *http.Request, m ErrorRes) {
	Encode(w, r, &m)
}
