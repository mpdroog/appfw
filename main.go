package main

import (
	"crypto/subtle"
	"flag"
	"fmt"
	"github.com/VojtechVitek/ratelimit"
	"github.com/VojtechVitek/ratelimit/memory"
	"github.com/coreos/go-systemd/daemon"
	"github.com/itshosted/webutils/middleware"
	"github.com/itshosted/webutils/muxdoc"
	"github.com/jinzhu/configor"
	"github.com/mpdroog/afd/config"
	"github.com/mpdroog/afd/handlers"
	ttl_map "github.com/mpdroog/afd/map"
	"github.com/mpdroog/afd/writer"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	mux     muxdoc.MuxDoc
	ln      net.Listener
	version = "dev-build"

	heap *ttl_map.Heap
)

func Init(f string) error {
	e := configor.New(&configor.Config{ENVPrefix: "APPFW"}).Load(&config.C, f)
	if e != nil {
		return e
	}

	if config.C.APIKey == "" {
		return fmt.Errorf("Missing config.APIKey")
	}

	heap = ttl_map.New(config.C.State, config.C.StateSize)
	if e := heap.Load(); e != nil {
		_ = heap.Delete()
		fmt.Printf("WARN: Flushed state as it was corrupt (e=%s)\n", e.Error())
	}
	if config.Verbose {
		heap.Range(func(key string, value interface{}, ttl int64, max int) {
			fmt.Printf("%s=%+v (%d)\n", key, value, ttl)
		})
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
	if config.Verbose {
		config.Verbose = false
		msg += "OFF"
	} else {
		config.Verbose = true
		msg += "ON"
	}
	msg += `"}`
	fmt.Printf("HTTP.Verbosity set to %t\n", config.Verbose)

	w.Header().Set("Content-Type", "application/json")
	if _, e := w.Write([]byte(msg)); e != nil {
		fmt.Printf("verbose: " + e.Error())
		return
	}
}

func check(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		w.WriteHeader(400)
		writer.Err(w, r, writer.ErrorRes{Error: "GET[key] missing", Detail: nil})
		return
	}
	// prefix
	key = "limit_" + key
	data, ok := heap.GetData(key)
	if !ok {
		// Not set, so ignore
		w.WriteHeader(204)
		return
	}

	val := data.Value
	max := data.Max
	if val >= max {
		// Limit reached
		w.WriteHeader(403)
		writer.Err(w, r, writer.ErrorRes{Error: "Limit reached", Detail: fmt.Sprintf("cur=%d max=%d", val, max)})
		return
	}

	// All fine, ignore
	w.WriteHeader(204)
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
	if strategy != "24UPDATE" && strategy != "24ADD" && strategy != "1UPDATE" {
		w.WriteHeader(400)
		writer.Err(w, r, writer.ErrorRes{Error: "GET[strategy] invalid, options=[24UPDATE,24ADD,1UPDATE]", Detail: nil})
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

	ttl := int64(86400)
	if strategy == "1UPDATE" {
		ttl = int64(3600)
	}

	if strategy == "24UPDATE" || strategy == "1UPDATE" {
		// Always set (increase TTL)
		e = heap.Set(key, val, ttl, max)
	} else if strategy == "24ADD" {
		// Only set value (ignore TTL if update)
		e = heap.SetValue(key, val, ttl, max)
	}

	if e != nil {
		fmt.Printf("appfw heap.Set=%s\n", e.Error())
		w.WriteHeader(500)
		writer.Err(w, r, writer.ErrorRes{Error: "heap.Set error", Detail: e.Error()})
		return
	}

	if val >= max {
		// TODO: Somehow save limit is reached so we can easily filter?
		w.WriteHeader(403)
		writer.Err(w, r, writer.ErrorRes{Error: "Limit reached", Detail: fmt.Sprintf("cur=%d max=%d", val, max)})
		return
	}

	w.WriteHeader(204)
}

func memfn(w http.ResponseWriter, r *http.Request) {
	apikey := r.URL.Query().Get("apikey")
	if apikey == "" || subtle.ConstantTimeCompare([]byte(apikey), []byte(config.C.APIKey)) != 1 {
		w.WriteHeader(401)
		writer.Err(w, r, writer.ErrorRes{Error: "Invalid GET[apikey]", Detail: nil})
		return
	}

	w.Header().Add("X-APPFW", version)
	if e := writer.Encode(w, r, heap.Fork()); e != nil {
		fmt.Printf("appfw.memory e=%s\n", e.Error())
		w.WriteHeader(403)
		writer.Err(w, r, writer.ErrorRes{Error: "Failed forking memory", Detail: nil})
	}
}

func memclear(w http.ResponseWriter, r *http.Request) {
	apikey := r.URL.Query().Get("apikey")
	if apikey == "" || subtle.ConstantTimeCompare([]byte(apikey), []byte(config.C.APIKey)) != 1 {
		w.WriteHeader(401)
		writer.Err(w, r, writer.ErrorRes{Error: "Invalid GET[apikey]", Detail: nil})
		return
	}

	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		w.WriteHeader(400)
		writer.Err(w, r, writer.ErrorRes{Error: "GET[pattern] missing", Detail: nil})
		return
	}

	if pattern == "*" {
		// Reset
		heap.Close()
		if e := heap.Delete(); e != nil {
			w.WriteHeader(400)
			writer.Err(w, r, writer.ErrorRes{Error: "Failed deleting current state", Detail: e.Error()})
			return
		}

		oldheap := heap
		heap = ttl_map.New(config.C.State, config.C.StateSize)
		if e := heap.Load(); e != nil {
			panic(e)
		}

		w.Header().Add("X-Affect", fmt.Sprintf("%d", oldheap.Len()))
		w.WriteHeader(204)
		return
	}

	affect := 0
	heap.Range(func(key string, value interface{}, ttl int64, max int) {
		if strings.Contains(key, pattern) {
			fmt.Printf("appfw.clear key=%s\n", key)
			heap.Del(key)
			affect++
		}
	})

	w.Header().Add("X-Affect", fmt.Sprintf("%d", affect))
	w.WriteHeader(204)
}

func cleanup(w http.ResponseWriter, r *http.Request) {
	apikey := r.URL.Query().Get("apikey")
	if apikey == "" || subtle.ConstantTimeCompare([]byte(apikey), []byte(config.C.APIKey)) != 1 {
		w.WriteHeader(401)
		writer.Err(w, r, writer.ErrorRes{Error: "Invalid GET[apikey]", Detail: nil})
		return
	}

	fmt.Printf("heap.Cleanup\n")
	if e := heap.Save(); e != nil {
		fmt.Printf("heap.Save e=%s\n", e.Error())
		w.WriteHeader(500)
		writer.Err(w, r, writer.ErrorRes{Error: "Failed saving heap", Detail: nil})
		return
	}

	nextheap := ttl_map.New(config.C.State, config.C.StateSize)
	if e := nextheap.Load(); e != nil {
		fmt.Printf("WARN: nextheap.Load failed e=%s\n", e.Error())
		w.WriteHeader(500)
		writer.Err(w, r, writer.ErrorRes{Error: "Failed reloading heap", Detail: nil})
		return
	}

	// swap
	oldheap := heap
	heap = nextheap

	if e := oldheap.Close(); e != nil {
		w.WriteHeader(500)
		fmt.Printf("WARN: oldheap.Close failed e=%s\n", e.Error())
		return
	}

	w.WriteHeader(204)
}

func main() {
	var (
		showVersion bool
		path        string
	)

	flag.BoolVar(&showVersion, "V", false, "Show version")
	flag.BoolVar(&config.Verbose, "v", false, "Show all that happens")
	flag.StringVar(&path, "c", "./config.toml", "Config-file")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if e := Init(path); e != nil {
		fmt.Printf("Init e=%s\n", e.Error())
		os.Exit(1)
		return
	}
	if config.Verbose {
		fmt.Printf("appfw=%+v\n", config.C)
	}

	if config.C.StateSize <= 0 {
		fmt.Printf("config.StateSize=0, rejecting!\n")
		os.Exit(1)
		return
	}

	mux.Title = "Appfw-API (v=" + version + ")"
	mux.Desc = "Application Firewall Daemon"
	mux.Add("/", doc, "This documentation")
	mux.Add("/verbose", verbose, "Toggle verbosity-mode")
	mux.Add("/check", check, "Check limit-counter (without increasing)")
	mux.Add("/limit", limit, "Increase limit-counter")
	mux.Add("/memory", memfn, "Dump current state to client")
	mux.Add("/clear", memclear, "Remove one or more entries")
	mux.Add("/cleanup", cleanup, "(Cleanp) Remove expired entries")

	var e error
	server := &http.Server{
		Addr:         config.C.Listen,
		TLSConfig:    DefaultTLSConfig(),
		Handler:      nil,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// Default handler
	var handler http.Handler = middleware.Use(mux.Mux)

	if config.C.Ratelimit == 0 {
		fmt.Printf("WARN: appfw ratelimit disabled by config\n")
	} else {
		// Extend handler with ratelimiter
		limit := ratelimit.Request(ratelimit.IP).Rate(config.C.Ratelimit, time.Minute).LimitBy(memory.New())
		handler = limit(handler)
	}

	// Set handler and add accesslog
	server.Handler = handlers.AccessLog(handler)

	ln, e = net.Listen("tcp", server.Addr)
	if e != nil {
		panic(e)
	}

	go func() {
		// Reset state-file every 24hours (to prevent it becoming too big)
		ticker := time.NewTicker(24 * time.Hour)

		for {
			<-ticker.C
			fmt.Printf("@daily heap.Save")
			if e := heap.Save(); e != nil {
				fmt.Printf("heap.Save e=%s\n", e.Error())
			}

			nextheap := ttl_map.New(config.C.State, 1024)
			if e := nextheap.Load(); e != nil {
				fmt.Printf("WARN: nextheap.Load failed\n")
				continue
			}

			// swap
			oldheap := heap
			heap = nextheap

			oldheap.Close()
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sent, e := daemon.SdNotify(false, "READY=1")
	if e != nil {
		panic(e)
	}
	if !sent {
		fmt.Printf("SystemD notify NOT sent\n")
	}

	closing := false
	go func() {
		<-sigs
		fmt.Printf("TERM signal\n")
		closing = true
		server.Close()
	}()

	if e := server.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)}); e != nil {
		if !closing {
			panic(e)
		}
	}

	// Finish state
	heap.Close()
	heap.Save()
}
