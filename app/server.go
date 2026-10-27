package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync"
)

// serve runs the UI over plain HTTP instead of a Wails window (--serve
// addr): the same Go backend, reached with fetch and server-sent events.
// Open http://addr/?http in any browser. Used for headless screenshots and
// browser tests; with --demo it needs no phone.
func (m *Messages) serve(addr string) error {
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return err
	}
	hub := &eventHub{clients: map[chan []byte]struct{}{}}
	m.httpEvents = hub
	go m.backend.start(context.Background())

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServerFS(dist))
	mux.Handle("/media/", mediaMiddleware(m)(http.NotFoundHandler()))
	mux.Handle("/attachment", mediaMiddleware(m)(http.NotFoundHandler()))
	mux.HandleFunc("/events", hub.serve)
	// The Wails runtime looks for this; there is none outside the app.
	mux.HandleFunc("/wails/custom.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
	})
	mux.HandleFunc("/rpc/{method}", m.rpc)
	log.Printf("serving the UI at http://%s/?http", addr)
	return http.ListenAndServe(addr, mux)
}

// rpc calls one Messages method by name with a JSON array of arguments.
// New exported Messages methods need a case here as well, or the browser
// (--serve, used by the UI tests) fails with "unknown method".
func (m *Messages) rpc(w http.ResponseWriter, r *http.Request) {
	var args []json.RawMessage
	_ = json.NewDecoder(r.Body).Decode(&args)
	arg := func(i int, v any) {
		if i < len(args) {
			_ = json.Unmarshal(args[i], v)
		}
	}
	var result any
	var err error
	switch r.PathValue("method") {
	case "Call":
		var cmd string
		params := map[string]any{}
		arg(0, &cmd)
		arg(1, &params)
		result, err = m.Call(cmd, params)
	case "IsDemo":
		result = m.IsDemo()
	case "GetSettings":
		result = m.GetSettings()
	case "UpdateSettings":
		var s Settings
		arg(0, &s)
		m.UpdateSettings(s)
	case "SetZoom":
		var z float64
		arg(0, &z)
		m.SetZoom(z)
	case "SignIn":
		err = m.SignIn()
	case "RequestNotifications", "BlurSupported", "ShareSupported":
		result = false
	case "SetAppearance", "SetBlur":
		// The browser follows its own color scheme.
	case "DropFiles":
		// The UI tests' stand-in for files dropped on the window.
		var paths []string
		arg(0, &paths)
		m.filesDropped(paths)
	case "Notify", "OpenPath", "SaveAs", "SaveContact", "ShareContact", "OpenDataFolder", "PickFile":
		// Desktop-only; nothing to do in a browser.
		result = ""
	default:
		err = fmt.Errorf("unknown method %q", r.PathValue("method"))
	}
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
}

// eventHub fans backend events out to browser tabs as server-sent events.
type eventHub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func (h *eventHub) publish(name string, data any) {
	line, err := json.Marshal(map[string]any{"name": name, "data": data})
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- line:
		default: // a stuck tab doesn't stall the backend
		}
	}
}

func (h *eventHub) serve(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	c := make(chan []byte, 256)
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
	}()
	flusher.Flush()
	for {
		select {
		case line := <-c:
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
