package main

import (
	"flag"
	"fmt"
	"github.com/VojtechVitek/ratelimit"
	"github.com/VojtechVitek/ratelimit/memory"
	"github.com/itshosted/webutils/middleware"
	"github.com/itshosted/webutils/muxdoc"
	"github.com/jinzhu/configor"
	ttl_map "github.com/mpdroog/afd/map"
	"github.com/mpdroog/afd/writer"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	/* Path/to/state */
	State string `default:"./state.tsv"`
	/** Host:port addr */
	Listen string `default:"127.0.0.1:1337"`
	/* Request limit per minute */
	Ratelimit int `default:"50"`
}

var (
	mux muxdoc.MuxDoc
	ln  net.Listener

	Verbose bool
	C       Config
	heap    *ttl_map.Heap
)

func Init(f string) error {
	e := configor.New(&configor.Config{ENVPrefix: "AFD"}).Load(&C, f)
	if e != nil {
		return e
	}

	heap = ttl_map.New()
	heap.Path(C.State)
	if _, e := os.Stat(C.State); e == nil {
		if e := heap.Restore(); e != nil {
			return e
		}
	}
	return nil
}

// Return API Documentation (paths)
func doc(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(mux.String()))
}

func verbose(w http.ResponseWriter, r *http.Request) {
	msg := `{"success": true, "msg": "Set verbosity to `
	if Verbose {
		Verbose = false
		msg += "OFF"
	} else {
		Verbose = true
		msg += "ON"
	}
	msg += `"}`
	fmt.Printf("HTTP.Verbosity set to %t\n", Verbose)

	w.Header().Set("Content-Type", "application/json")
	if _, e := w.Write([]byte(msg)); e != nil {
		fmt.Printf("verbose: " + e.Error())
		return
	}
}

func limit(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		w.WriteHeader(400)
		writer.Err(w, r, writer.ErrorRes{Error: "GET[key] missing", Detail: nil})
		return
	}
	// prefix
	key = "limit_" + key

	maxStr := r.URL.Query().Get("max")
	if maxStr == "" {
		w.WriteHeader(400)
		writer.Err(w, r, writer.ErrorRes{Error: "GET[max] missing", Detail: nil})
		return
	}
	strategy := r.URL.Query().Get("strategy")
	if strategy != "24UPDATE" && strategy != "24ADD" {
		w.WriteHeader(400)
		writer.Err(w, r, writer.ErrorRes{Error: "GET[strategy] invalid, options=[24UPDATE,24ADD]", Detail: nil})
		return
	}

	max, e := strconv.Atoi(maxStr)
	if e != nil {
		w.WriteHeader(400)
		writer.Err(w, r, writer.ErrorRes{Error: "GET[max] not number", Detail: nil})
		return
	}

	val := heap.GetInt(key)
	val++

	if val >= max {
		if strategy == "24UPDATE" {
			heap.Set(key, val, 86400) // Increase TTL
		}

		w.WriteHeader(403)
		writer.Err(w, r, writer.ErrorRes{Error: "Limit reached", Detail: nil})
		return
	}

	heap.Set(key, val, 86400)
	w.WriteHeader(204)
}

// TODO: Limit with auth?
func memfn(w http.ResponseWriter, r *http.Request) {
	if e := writer.Encode(w, r, heap.Fork()); e != nil {
		fmt.Printf("AFD.memory e=%s\n", e.Error())
		w.WriteHeader(403)
		writer.Err(w, r, writer.ErrorRes{Error: "Failed forking memory", Detail: nil})
	}
}

// TODO: Limit with auth?
func memclear(w http.ResponseWriter, r *http.Request) {
	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		w.WriteHeader(400)
		writer.Err(w, r, writer.ErrorRes{Error: "GET[pattern] missing", Detail: nil})
		return
	}

	if pattern == "*" {
		// Reset
		heap = ttl_map.New()
		heap.Path(C.State)

		if e := os.Remove(C.State); e != nil {
			w.WriteHeader(400)
			writer.Err(w, r, writer.ErrorRes{Error: "Failed deleting current state", Detail: e.Error()})
			return
		}

		w.WriteHeader(204)
		return
	}

	heap.Range(func(key string, value interface{}, ttl int64) {
		if strings.Contains(key, pattern) {
			fmt.Printf("AFD.clear key=%s\n", key)
			heap.Del(key)
		}
	})
}

func main() {
	var path string
	flag.BoolVar(&Verbose, "v", false, "Show all that happens")
	flag.StringVar(&path, "c", "./config.toml", "Config-file")
	flag.Parse()

	if e := Init(path); e != nil {
		panic(e)
	}

	mux.Title = "AFD-API"
	mux.Desc = "Application Firewall Daemon"
	mux.Add("/", doc, "This documentation")
	mux.Add("/verbose", verbose, "Toggle verbosity-mode")
	mux.Add("/limit", limit, "Increase limit-counter")
	mux.Add("/memory", memfn, "Dump current state to client")
	mux.Add("/clear", memclear, "Remove one or more entries")

	var e error
	// Max Nreq/min against bruteforcing
	limit := ratelimit.Request(ratelimit.IP).Rate(C.Ratelimit, time.Minute).LimitBy(memory.New())
	server := &http.Server{
		Addr:         C.Listen,
		TLSConfig:    DefaultTLSConfig(),
		Handler:      limit(middleware.Use(mux.Mux)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	ln, e = net.Listen("tcp", server.Addr)
	if e != nil {
		panic(e)
	}
	if Verbose {
		fmt.Printf("AFD=%+v\n", C)
	}

	if e := server.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)}); e != nil {
		panic(e)
	}
}
