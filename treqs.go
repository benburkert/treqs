// Package treqs provides tracing for individual HTTP requests.
//
// Incoming requests pass through the Tracer handler. If the request contains
// valid key & action headers (X-Treqs-Key & X-Treqs-Action) the runtime tracer
// is enabled for the life of the request. While the tracer is enabled an
// exclusive lock is held so that only the single request is traced. All other
// requests hold a shared lock.
//
// To use treqs wrap the application http.Handler with a Tracer.
//
//	var handler http.Handler
//	// ...
//	tracer := &treqs.Tracer{
//		Key:     "secret-treqs-key",
//		Handler: handler,
//	}
//
// Trace a request by adding the action & key headers.
//
//	var req *http.Request
//	// ...
//	req.Header.Set("X-Treqs-Action", "trace")
//	req.Header.Set("X-Treqs-Key", "secret-treqs-key")
//	res, _ := http.DefaultClient.Do(req)
//
// Download the trace data for the last request with the action,
// key, and ID headers.
//
//	traceID := res.Header.Get("X-Treqs-Id")
//
//	req, _ = http.NewRequest("GET", req.URL.String(), nil)
//	req.Header.Set("X-Treqs-Action", "read")
//	req.Header.Set("X-Treqs-Id", traceID)
//	req.Header.Set("X-Treqs-Key", "secret-treqs-key")
//
//	var file *os.File
//	// ...
//
//	res, _ := http.DefaultClient.Do(req)
//	io.Copy(file, res.Body)
//
// The "treqs" command can be used instead to issue a traced request
// and writes the contents to stdout.
//
//	$ go get -u github.com/benburkert/treqs/cmd/treqs
//	$ treqs -url http://localhost -key secret-treqs-key > trace.out
//	$ go tools trace treqs trace.out
//
// Goroutines that live outside of HTTP server may be captured in
// trace data. The Exclude method prevents functions from inclusion
// in the trace.
//
//	tracer := &treqs.Tracer{
//		Key:     "secret-treqs-key",
//		Handler: app,
//	}
//	go http.ListenAndServe(addr, tracer)
//
//	for := range time.NewTicker(30*time.Second) {
//		tracer.Exclude(app.Compact)
//	}
//
package treqs

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"runtime/trace"
	"strings"
	"sync"
)

const (
	xTReqsAction = "X-Treqs-Action"
	xTReqsID     = "X-Treqs-Id"
	xTReqsKey    = "X-Treqs-Key"
)

// Tracer is a http.Handler for enabling runtime tracing on a wrapped
// http.Handler. The handler supports three actions: trace, read, &
// reset.
type Tracer struct {
	http.Handler

	Key string

	mu     sync.RWMutex
	traces map[string]*bytes.Buffer
}

// Exclude prevents the func from inclusion in a trace.
func (t *Tracer) Exclude(fn func()) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	fn()
}

func (t *Tracer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key, action, id := scrubHeader(r.Header)

	if key != t.Key {
		action = ""
	}

	switch strings.ToLower(action) {
	default:
		t.mu.RLock()
		defer t.mu.RUnlock()

		t.Handler.ServeHTTP(w, r)
	case "read":
		t.read(id, w, r)
	case "reset":
		t.reset(w, r)
	case "trace":
		t.trace(w, r)
	}
}

func (t *Tracer) read(id string, w http.ResponseWriter, r *http.Request) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	buf, ok := t.traces[id]
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(buf.Bytes())
}

func (t *Tracer) reset(w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.traces = make(map[string]*bytes.Buffer)
}

func (t *Tracer) trace(w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	t.mu.Unlock()

	id, buf := randHex(), bytes.NewBuffer(nil)
	w.Header().Set(xTReqsID, id)

	if err := trace.Start(buf); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Could not enable tracing: %s\n", err)
		return
	}

	t.Handler.ServeHTTP(w, r)
	trace.Stop()

	t.traces[id] = buf
}

func randHex() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic(err)
	}

	return hex.EncodeToString(b)
}

func scrubHeader(hdr http.Header) (key, action, id string) {
	for k := range hdr {
		switch k {
		case xTReqsKey:
			if key == "" {
				key = hdr.Get(k)
			}
		case xTReqsAction:
			if action == "" {
				action = hdr.Get(k)
			}
		case xTReqsID:
			if id == "" {
				id = hdr.Get(k)
			}
		}

		delete(hdr, k)
	}
	return
}
