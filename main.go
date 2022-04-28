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
	"net"
	"net/http"
	"strconv"
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
	if e := heap.Restore(); e != nil {
		return e
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
		w.Write([]byte("GET[key] missing"))
		return
	}
	// prefix
	key = "limit_" + key

	maxStr := r.URL.Query().Get("max")
	if maxStr == "" {
		w.WriteHeader(400)
		w.Write([]byte("GET[max] missing"))
		return
	}
	strategy := r.URL.Query().Get("strategy")
	if strategy != "24UPDATE" && strategy != "24ADD" {
		w.WriteHeader(400)
		w.Write([]byte("GET[strategy] invalid, options=[24UPDATE,24ADD]"))
		return
	}

	max, e := strconv.Atoi(maxStr)
	if e != nil {
		w.WriteHeader(400)
		w.Write([]byte("ERR: GET[max] not number"))
		return
	}

	val := heap.GetInt(key)
	val++

	if strategy == "24UPDATE" {
		heap.Set(key, val, 86400) // Increase TTL
	}

	if val >= max {
		w.WriteHeader(423)
		w.Write([]byte("LIMIT reached"))
		return
	}

	heap.Set(key, val, 86400)

	w.Write([]byte("OK"))
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
