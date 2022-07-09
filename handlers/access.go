// Accesslog
package handlers

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/mpdroog/afd/config"
	"net/http"
	"time"
)

type Msg struct {
	Method    string
	Host      string
	URL       string
	Status    int
	Remote    string
	Ratelimit string
	Duration  int64
	UA        string
	Proto     string
	Len       uint64
	Date      string
	Time      string
	Referer   string
}

type statusWriter struct {
	http.ResponseWriter
	Status int
	Length uint64
}

func (w *statusWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *statusWriter) WriteHeader(status int) {
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.Status == 0 {
		w.Status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.Length += uint64(n)
	return n, err
}

func AccessLog(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		h.ServeHTTP(sw, r)

		if !config.Verbose {
			return
		}

		// TODO: Re-use objects?
		diff := time.Since(begin)
		msg := Msg{}
		msg.Method = r.Method
		msg.Host = r.Host
		msg.URL = r.URL.String()
		msg.Status = sw.Status
		msg.Remote = r.RemoteAddr
		msg.Ratelimit = w.Header().Get("X-Ratelimit-Remaining")
		msg.Duration = int64(diff.Seconds())
		msg.UA = r.Header.Get("User-Agent")
		msg.Proto = r.Proto
		msg.Len = sw.Length
		msg.Date = begin.Format("2006-01-02")
		msg.Time = begin.Format("15:04:05")
		msg.Referer = r.Referer()

		txt := fmt.Sprintf("[%s %s] (status=%d) appfw.http(%s %s%s)\n", msg.Date, msg.Time, msg.Status, msg.Method, msg.Host, msg.URL)
		if msg.Status != 204 && msg.Status != 200 {
			d := color.New(color.FgRed, color.Bold)
			d.Printf(txt)
		} else {
			fmt.Printf(txt)
		}
	})
}
