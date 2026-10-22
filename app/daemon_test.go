package main

import (
	"context"
	"errors"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/mautrix-gmessages/pkg/libgm/gmproto"
)

// ---- helpers ----

type recorded struct {
	name string
	data any
}

// recorder captures what the daemon sends to the UI.
type recorder struct {
	mu     sync.Mutex
	events []recorded
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recorded{name, data})
}

func (r *recorder) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.name)
	}
	return out
}

// states lists the link states sent to the UI, in order.
func (r *recorder) states() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		if e.name == "state" {
			out = append(out, e.data.(map[string]string)["state"])
		}
	}
	return out
}

func testDaemon(t *testing.T) (*daemon, *recorder) {
	t.Helper()
	r := &recorder{}
	return newDaemon(&out{emit: r.emit}, t.TempDir(), zerolog.Nop()), r
}

// fastBackoff makes connect retries immediate for the test.
func fastBackoff(t *testing.T) {
	t.Helper()
	saved := connectBackoff
	connectBackoff = func(int) time.Duration { return time.Millisecond }
	t.Cleanup(func() { connectBackoff = saved })
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// ---- the deadlock (regression) ----

// payload is one item off the long poll.
type payload struct {
	response bool // a reply to our request: libgm routes it, no event
	name     string
}

// longPoll mimics libgm's reader: one goroutine reads payloads in order.
// Replies to requests resolve the waiting request directly; everything else
// goes to the event handler, and the next payload isn't read until the
// handler returns.
func longPoll(stream <-chan payload, handler func(any), resolve func()) {
	for p := range stream {
		if p.response {
			resolve()
			continue
		}
		handler(p.name)
	}
}

// startup replays what the debug log showed: a message arrives first, its
// handler asks the phone for the conversation (GetConversation), and the
// phone's answer is the next payload on the same long poll. It returns a
// channel closed once the handler got its answer.
func startup(t *testing.T, wrap func(func(any)) func(any)) <-chan struct{} {
	t.Helper()
	answered := make(chan struct{})
	reply := make(chan struct{})
	var once sync.Once
	resolve := func() { once.Do(func() { close(reply) }) }
	// Unblocks the handler at the end if the reader never could.
	t.Cleanup(resolve)

	handle := func(evt any) {
		if evt == "message" {
			<-reply // waiting on the phone, like participants() → GetConversation
			select {
			case <-answered:
			default:
				close(answered)
			}
		}
	}
	stream := make(chan payload, 2)
	stream <- payload{name: "message"}
	stream <- payload{response: true}
	close(stream)
	go longPoll(stream, wrap(handle), resolve)
	return answered
}

// The bug: with the handler called on the reader, the reply is never read.
// This pins down that the test reproduces the deadlock, so the fix below is
// proven against the real failure and not a test that always passes.
func TestHandlerOnReaderDeadlocks(t *testing.T) {
	direct := func(h func(any)) func(any) { return h }
	select {
	case <-startup(t, direct):
		t.Fatal("expected the direct handler to deadlock, but the reply got through")
	case <-time.After(300 * time.Millisecond):
	}
}

// The fix: queued, the reader keeps going and delivers the reply.
func TestQueuedHandlerDoesNotBlockReader(t *testing.T) {
	queued := func(h func(any)) func(any) { return queueEvents(eventQueueSize, h, zerolog.Nop()) }
	waitFor(t, startup(t, queued), "the handler to get the phone's reply")
}

// A full queue must not block the reader either.
func TestQueueFullStillDoesNotBlock(t *testing.T) {
	gate := make(chan struct{})
	var handled atomic.Int32
	enqueue := queueEvents(1, func(any) {
		<-gate
		handled.Add(1)
	}, zerolog.Nop())

	sent := make(chan struct{})
	go func() {
		for i := range 5 {
			enqueue(i)
		}
		close(sent)
	}()
	waitFor(t, sent, "enqueue to return while the handler is stuck")

	close(gate)
	deadline := time.Now().Add(2 * time.Second)
	for handled.Load() < 5 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := handled.Load(); got != 5 {
		t.Fatalf("handled %d of 5 events", got)
	}
}

// Happy path: events are handled in the order libgm sent them.
func TestQueueKeepsOrder(t *testing.T) {
	const n = 1000
	var mu sync.Mutex
	var got []int
	done := make(chan struct{})
	enqueue := queueEvents(eventQueueSize, func(evt any) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, evt.(int))
		if len(got) == n {
			close(done)
		}
	}, zerolog.Nop())
	for i := range n {
		enqueue(i)
	}
	waitFor(t, done, "all events")
	for i, v := range got {
		if v != i {
			t.Fatalf("event %d handled at position %d", v, i)
		}
	}
}

// ---- connecting ----

// Happy path: the link shows connected as soon as Connect succeeds. Before,
// it waited for libgm's ClientReady, which never fires, so the UI could sit
// on "connecting" with no conversations.
func TestConnectSucceedsFirstTry(t *testing.T) {
	d, r := testDaemon(t)
	calls := 0
	ok := d.connectWithRetry(context.Background(), func(context.Context) error {
		calls++
		return nil
	})
	if !ok || calls != 1 {
		t.Fatalf("ok=%v calls=%d, want true and 1", ok, calls)
	}
	if got := r.states(); !slices.Equal(got, []string{stateConnected}) {
		t.Fatalf("states %v, want [connected]", got)
	}
}

func TestConnectRetriesTransientErrors(t *testing.T) {
	fastBackoff(t)
	d, r := testDaemon(t)
	calls := 0
	ok := d.connectWithRetry(context.Background(), func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("network is unreachable")
		}
		return nil
	})
	if !ok || calls != 3 {
		t.Fatalf("ok=%v calls=%d, want true and 3", ok, calls)
	}
	if got := r.states(); !slices.Equal(got, []string{stateConnected}) {
		t.Fatalf("states %v, want [connected]", got)
	}
}

func TestConnectRejectedSessionLogsOut(t *testing.T) {
	fastBackoff(t)
	d, r := testDaemon(t)
	if err := os.WriteFile(d.sessionPath(), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	ok := d.connectWithRetry(context.Background(), func(context.Context) error {
		calls++
		return errors.New("failed to refresh auth token: 401")
	})
	if ok || calls != 3 {
		t.Fatalf("ok=%v calls=%d, want false and 3", ok, calls)
	}
	if !slices.Contains(r.names(), "logged_out") {
		t.Fatalf("events %v, want logged_out", r.names())
	}
	if got := r.states(); !slices.Equal(got, []string{stateUnpaired}) {
		t.Fatalf("states %v, want [unpaired]", got)
	}
	if _, err := os.Stat(d.sessionPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("session file still there: %v", err)
	}
}

func TestConnectStopsWhenCancelled(t *testing.T) {
	saved := connectBackoff
	connectBackoff = func(int) time.Duration { return time.Hour }
	t.Cleanup(func() { connectBackoff = saved })

	d, r := testDaemon(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		if d.connectWithRetry(ctx, func(context.Context) error { return errors.New("offline") }) {
			t.Error("connected after cancel")
		}
		close(done)
	}()
	cancel()
	waitFor(t, done, "connect to give up")
	if got := r.states(); len(got) != 0 {
		t.Fatalf("states %v, want none", got)
	}
}

// ---- state and participants ----

func TestSetStateOnlySendsChanges(t *testing.T) {
	d, r := testDaemon(t)
	for _, s := range []string{stateConnecting, stateConnecting, stateConnected, stateConnected, statePhoneOffline} {
		d.setState(s)
	}
	want := []string{stateConnecting, stateConnected, statePhoneOffline}
	if got := r.states(); !slices.Equal(got, want) {
		t.Fatalf("states %v, want %v", got, want)
	}
}

// Happy path: a cached conversation answers without asking the phone.
func TestParticipantsFromCache(t *testing.T) {
	d, _ := testDaemon(t)
	d.cacheConversations([]*gmproto.Conversation{{
		ConversationID: "c1",
		Participants: []*gmproto.Participant{
			participant("me", "+15550001", "Me", true, true),
			participant("sam", "+15550002", "Sam", false, true),
		},
	}})
	got := d.participants(context.Background(), "c1")
	if len(got) != 2 || got["sam"].GetFullName() != "Sam" {
		t.Fatalf("participants %v", got)
	}
}

// Not cached and no client: no names, and no waiting.
func TestParticipantsNotConnected(t *testing.T) {
	d, _ := testDaemon(t)
	start := time.Now()
	got := d.participants(context.Background(), "unknown")
	if len(got) != 0 {
		t.Fatalf("participants %v, want none", got)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("participants waited without a client")
	}
}
