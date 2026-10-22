package main

import (
	"context"
	"encoding/json"
)

// out delivers the backend's events (state, messages, typing, pairing…) to
// the UI. The backend is gogo-messages' sidecar code, unchanged: there it
// wrote JSON lines to stdout, here each event becomes a Wails event.
type out struct {
	emit func(name string, data any)
}

func (o *out) event(name string, data any) {
	if o.emit != nil {
		o.emit(name, data)
	}
}

// backend is implemented by the real libgm daemon and by the demo.
type backend interface {
	start(ctx context.Context)
	handle(ctx context.Context, cmd string, args json.RawMessage) (any, error)
}
