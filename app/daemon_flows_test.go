package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mau.fi/mautrix-gmessages/pkg/libgm"
	"go.mau.fi/mautrix-gmessages/pkg/libgm/events"
	"go.mau.fi/mautrix-gmessages/pkg/libgm/gmproto"
)

// Flows through the daemon without a phone: libgm events in, UI events out,
// and the commands the UI sends.

func (r *recorder) last(name string) (any, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.events) - 1; i >= 0; i-- {
		if r.events[i].name == name {
			return r.events[i].data, true
		}
	}
	return nil, false
}

func textMessage(id, conv, from, text string) *gmproto.Message {
	return &gmproto.Message{
		MessageID:      id,
		ConversationID: conv,
		ParticipantID:  from,
		Timestamp:      1_700_000_000_000_000,
		MessageStatus:  &gmproto.MessageStatus{Status: gmproto.MessageStatusType_INCOMING_COMPLETE},
		MessageInfo: []*gmproto.MessageInfo{{Data: &gmproto.MessageInfo_MessageContent{
			MessageContent: &gmproto.MessageContent{Content: text},
		}}},
	}
}

func samConversation() *gmproto.Conversation {
	return &gmproto.Conversation{
		ConversationID:    "c1",
		Name:              "Sam",
		DefaultOutgoingID: "me",
		Participants: []*gmproto.Participant{
			participant("me", "+15550001", "Me", true, true),
			participant("sam", "+15550002", "Sam", false, true),
		},
	}
}

// ---- link state ----

func TestLinkStateFromEvents(t *testing.T) {
	cases := []struct {
		name string
		evt  any
		want string
	}{
		{"phone stops answering", &events.PhoneNotResponding{}, statePhoneOffline},
		{"phone answers again", &events.PhoneRespondingAgain{}, stateConnected},
		{"long poll recovers", &events.ListenRecovered{}, stateConnected},
		{"long poll hiccup", &events.ListenTemporaryError{Error: errors.New("EOF")}, stateConnecting},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, r := testDaemon(t)
			d.setState(stateUnpaired) // a known start that differs from every target
			d.onEvent(context.Background(), nil, tc.evt)
			if got := r.states(); got[len(got)-1] != tc.want {
				t.Fatalf("state %v, want %s", got, tc.want)
			}
		})
	}
}

// The usual life of a session: up, phone dozes, phone back.
func TestPhoneDropsAndComesBack(t *testing.T) {
	d, r := testDaemon(t)
	d.connectWithRetry(context.Background(), func(context.Context) error { return nil })
	d.onEvent(context.Background(), nil, &events.PhoneNotResponding{})
	d.onEvent(context.Background(), nil, &events.PhoneRespondingAgain{})
	want := []string{stateConnected, statePhoneOffline, stateConnected}
	if got := r.states(); !slices.Equal(got, want) {
		t.Fatalf("states %v, want %v", got, want)
	}
}

func TestSessionEndingEventsLogOut(t *testing.T) {
	cases := []struct {
		name   string
		evt    any
		reason string
	}{
		{"signed out of Google", &events.GaiaLoggedOut{}, "Signed out of Google."},
		{"unpaired on the phone", &gmproto.RevokePairData{}, "Unpaired from the phone."},
		{"fatal listen error", &events.ListenFatalError{Error: errors.New("401")}, "Connection rejected: 401"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, r := testDaemon(t)
			for _, p := range []string{d.sessionPath(), d.contactsPath()} {
				if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			d.cacheConversations([]*gmproto.Conversation{samConversation()})
			d.setState(stateConnected)

			d.onEvent(context.Background(), nil, tc.evt)

			data, ok := r.last("logged_out")
			if !ok || data.(map[string]string)["reason"] != tc.reason {
				t.Fatalf("logged_out %v, want reason %q", data, tc.reason)
			}
			if got := r.states(); got[len(got)-1] != stateUnpaired {
				t.Fatalf("states %v, want unpaired last", got)
			}
			for _, p := range []string{d.sessionPath(), d.contactsPath()} {
				if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("%s still there", p)
				}
			}
			if len(d.convs) != 0 {
				t.Fatal("conversation cache not cleared")
			}
		})
	}
}

// ---- incoming messages ----

// The daemon side of the deadlock: a message for a conversation we haven't
// cached yet must still reach the UI, without names, and without waiting.
func TestMessageForUnknownConversationIsDeliveredAtOnce(t *testing.T) {
	d, r := testDaemon(t)
	start := time.Now()
	d.onEvent(context.Background(), nil, &libgm.WrappedMessage{Message: textMessage("m1", "unknown", "sam", "hi")})
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("handling took %s", time.Since(start))
	}
	data, ok := r.last("message")
	if !ok {
		t.Fatal("no message event")
	}
	msg := data.(Message)
	if msg.ID != "m1" || msg.Text != "hi" || msg.Sender != "" {
		t.Fatalf("message %+v", msg)
	}
}

func TestMessageForCachedConversationHasSender(t *testing.T) {
	d, r := testDaemon(t)
	d.onEvent(context.Background(), nil, samConversation())
	d.onEvent(context.Background(), nil, &libgm.WrappedMessage{Message: textMessage("m1", "c1", "sam", "hi"), IsOld: true})
	data, _ := r.last("message")
	msg := data.(Message)
	if msg.Sender != "Sam" || msg.FromMe || !msg.IsOld || msg.ConversationID != "c1" {
		t.Fatalf("message %+v", msg)
	}
}

func TestConversationEventIsSentAndCached(t *testing.T) {
	d, r := testDaemon(t)
	d.onEvent(context.Background(), nil, samConversation())
	data, ok := r.last("conversation")
	if !ok || data.(Conversation).ID != "c1" {
		t.Fatalf("conversation event %v", data)
	}
	if d.convs["c1"] == nil {
		t.Fatal("conversation not cached")
	}
}

func TestTypingEvents(t *testing.T) {
	d, r := testDaemon(t)
	for _, typing := range []bool{true, false} {
		typ := gmproto.TypingTypes(0)
		if typing {
			typ = gmproto.TypingTypes_STARTED_TYPING
		}
		d.onEvent(context.Background(), nil, &gmproto.TypingData{ConversationID: "c1", Type: typ})
		data, _ := r.last("typing")
		got := data.(map[string]any)
		if got["conversation_id"] != "c1" || got["typing"] != typing {
			t.Fatalf("typing event %v, want typing=%v", got, typing)
		}
	}
}

// ---- SIMs ----

func simCard(number int32, carrier, participantID string) *gmproto.SIMCard {
	return &gmproto.SIMCard{
		SIMData: &gmproto.SIMData{
			SIMPayload:           &gmproto.SIMPayload{SIMNumber: number},
			CarrierName:          carrier,
			FormattedPhoneNumber: fmt.Sprintf("+1 555 000 %04d", number),
		},
		SIMParticipant: &gmproto.SIMParticipant{ID: participantID},
	}
}

func TestSettingsEventSendsUsableSIMs(t *testing.T) {
	d, r := testDaemon(t)
	d.onEvent(context.Background(), nil, &gmproto.Settings{SIMCards: []*gmproto.SIMCard{
		simCard(1, "Orange", "p1"),
		simCard(2, "", "p2"),    // no carrier name: labelled by number
		simCard(3, "Ghost", ""), // no participant: can't send from it
	}})
	data, ok := r.last("sims")
	if !ok {
		t.Fatal("no sims event")
	}
	sims := data.([]map[string]any)
	if len(sims) != 2 || sims[0]["label"] != "Orange" || sims[1]["label"] != "SIM 2" {
		t.Fatalf("sims %v", sims)
	}
	// The UI asking later gets the same list.
	got, err := d.handle(context.Background(), "sims", nil)
	if err != nil || len(got.([]map[string]any)) != 2 {
		t.Fatalf("sims command %v, %v", got, err)
	}
}

func TestSettingsWithoutSIMsKeepsPreviousList(t *testing.T) {
	d, r := testDaemon(t)
	d.onEvent(context.Background(), nil, &gmproto.Settings{SIMCards: []*gmproto.SIMCard{simCard(1, "Orange", "p1")}})
	d.onEvent(context.Background(), nil, &gmproto.Settings{})
	n := 0
	for _, name := range r.names() {
		if name == "sims" {
			n++
		}
	}
	if n != 1 || d.settings.GetSIMCards() == nil {
		t.Fatalf("sims events %d, settings %v", n, d.settings)
	}
}

// ---- commands ----

func TestStateCommand(t *testing.T) {
	d, _ := testDaemon(t)
	d.account = "jb@example.com"
	d.setState(stateConnected)
	got, err := d.handle(context.Background(), "state", nil)
	want := map[string]string{"state": stateConnected, "account": "jb@example.com"}
	if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("state %v, %v", got, err)
	}
}

// Everything that needs the phone answers "not connected" instead of
// hanging or crashing while the link is down.
func TestPhoneCommandsFailCleanlyWhenNotConnected(t *testing.T) {
	d, _ := testDaemon(t)
	for _, cmd := range []string{"conversations", "messages", "send", "contacts", "mark_read", "typing", "react",
		"delete_message", "update_conversation", "download", "avatars", "new_conversation", "no_such_command"} {
		if _, err := d.handle(context.Background(), cmd, json.RawMessage(`{"conversation_id":"c1"}`)); !errors.Is(err, errNotConnected) {
			t.Errorf("%s: err %v, want errNotConnected", cmd, err)
		}
	}
}

func TestBadArgumentsAreRejected(t *testing.T) {
	d, _ := testDaemon(t)
	if _, err := d.handle(context.Background(), "state", json.RawMessage(`{not json`)); err == nil {
		t.Fatal("bad JSON accepted")
	}
}

func TestLogoutWhileDisconnected(t *testing.T) {
	d, r := testDaemon(t)
	if err := os.WriteFile(d.sessionPath(), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	d.setState(stateConnecting)
	if _, err := d.handle(context.Background(), "logout", nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(r.names(), "logged_out") || r.states()[len(r.states())-1] != stateUnpaired {
		t.Fatalf("events %v", r.names())
	}
	if _, err := os.Stat(d.sessionPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("session file still there")
	}
}

func TestContactsCacheRoundTrip(t *testing.T) {
	d, _ := testDaemon(t)
	if got, _ := d.handle(context.Background(), "contacts_cached", nil); got.([]map[string]any) != nil {
		t.Fatalf("no file should give nil, got %v", got)
	}
	d.saveContacts([]map[string]any{{"name": "Sam", "number": "+15550002", "top": true}})
	got, err := d.handle(context.Background(), "contacts_cached", nil)
	contacts := got.([]map[string]any)
	if err != nil || len(contacts) != 1 || contacts[0]["name"] != "Sam" || contacts[0]["top"] != true {
		t.Fatalf("contacts %v, %v", got, err)
	}
	// A corrupt cache is ignored, not an error.
	if err := os.WriteFile(d.contactsPath(), []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := d.cachedContacts(); got != nil {
		t.Fatalf("corrupt cache gave %v", got)
	}
}

// ---- pairing ----

func TestPairWithoutCookiesFails(t *testing.T) {
	d, r := testDaemon(t)
	if _, err := d.handle(context.Background(), "pair_google", json.RawMessage(`{"cookies":{}}`)); err == nil {
		t.Fatal("pairing without cookies accepted")
	}
	if len(r.states()) != 0 {
		t.Fatalf("state changed: %v", r.states())
	}
}

func TestCancelPairing(t *testing.T) {
	d, r := testDaemon(t)
	ctx := d.beginPairing()
	if got := r.states(); !slices.Equal(got, []string{statePairing}) {
		t.Fatalf("states %v", got)
	}
	if _, err := d.handle(context.Background(), "cancel_pair", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("pairing context not cancelled")
	}
}

func TestPairingErrorsReadForPeople(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{libgm.ErrNoDevicesFound, "No phone found"},
		{libgm.ErrIncorrectEmoji, "wrong emoji"},
		{libgm.ErrPairingCancelled, "cancelled on the phone"},
		{libgm.ErrPairingTimeout, "didn't respond in time"},
		{fmt.Errorf("finish: %w", libgm.ErrPairingTimeout), "didn't respond in time"}, // wrapped
		{context.Canceled, "Pairing cancelled."},
		{errors.New("something new"), "something new"}, // unknown: shown as is
	}
	for _, tc := range cases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			d, r := testDaemon(t)
			d.pairFailed(tc.err)
			data, _ := r.last("pair_error")
			if msg := data.(map[string]string)["message"]; !strings.Contains(msg, tc.want) {
				t.Fatalf("message %q, want it to contain %q", msg, tc.want)
			}
			if got := r.states(); got[len(got)-1] != stateUnpaired {
				t.Fatalf("states %v", got)
			}
		})
	}
}

// ---- session on disk ----

func TestSessionRoundTrip(t *testing.T) {
	d, _ := testDaemon(t)
	ad := libgm.NewAuthData()
	ad.SessionID = uuid.New()
	ad.Cookies = map[string]string{"SID": "abc"}
	d.saveSession(ad)

	got, err := d.loadSession()
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != ad.SessionID || got.Cookies["SID"] != "abc" {
		t.Fatalf("loaded %v %v", got.SessionID, got.Cookies)
	}
	info, err := os.Stat(d.sessionPath())
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no Unix permission bits; files report 0666 regardless.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("session file mode %v", info.Mode().Perm())
	}
	if _, err := os.Stat(d.sessionPath() + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("temp file left behind")
	}
}

func TestSessionWithoutIDGetsOne(t *testing.T) {
	d, _ := testDaemon(t)
	if err := os.WriteFile(d.sessionPath(), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	ad, err := d.loadSession()
	if err != nil || ad.SessionID == uuid.Nil {
		t.Fatalf("session %v, %v", ad, err)
	}
}

// No session, or one we can't read: the app shows pairing, never connects.
func TestStartWithoutUsableSessionShowsPairing(t *testing.T) {
	for name, content := range map[string]*string{"missing": nil, "corrupt": ptr("{not json")} {
		t.Run(name, func(t *testing.T) {
			d, r := testDaemon(t)
			if content != nil {
				if err := os.WriteFile(d.sessionPath(), []byte(*content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			d.start(context.Background())
			if got := r.states(); !slices.Equal(got, []string{stateUnpaired}) {
				t.Fatalf("states %v, want [unpaired]", got)
			}
			if d.client != nil {
				t.Fatal("started a client")
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
