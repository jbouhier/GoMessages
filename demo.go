package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// demoAssets are the demo's contact photos and shared pictures: AI-generated
// portraits (CC0, faker-js/assets-person-portrait) and Unsplash photos.
//
//go:embed assets/demo/*.jpg
var demoAssets embed.FS

// demo serves sample data through the same protocol, so the app can be
// developed and screenshotted without a phone. Pairing pretends to succeed
// after a few seconds.
type demo struct {
	o   *out
	dir string

	mu     sync.Mutex
	state  string
	convs  map[string]*Conversation
	msgs   map[string][]Message
	nextID int
}

func newDemo(o *out, dir string) *demo {
	return &demo{o: o, dir: dir, convs: map[string]*Conversation{}, msgs: map[string][]Message{}}
}

func (d *demo) setState(s string) {
	d.mu.Lock()
	d.state = s
	d.mu.Unlock()
	d.o.event("state", map[string]string{"state": s, "account": "demo@gmail.com"})
}

func (d *demo) start(ctx context.Context) {
	now := time.Now()
	seed := []struct {
		id, name, color string
		group           bool
		lines           []struct {
			me  bool
			who string
			ago time.Duration
			t   string
		}
	}{
		{"1", "JB Bouhier", "#e8710a", false, nil},
		{"2", "Haruto Sato", "#1a73e8", false, nil},
		{"3", "Lukas Weber", "#188038", false, nil},
		{"4", "Oliver Bennett", "#a142f4", false, nil},
		{"5", "Mei Tanaka", "#5f6368", false, nil},
		{"6", "Lucía Fernández", "#d93025", false, nil},
	}
	type line = struct {
		me  bool
		who string
		ago time.Duration
		t   string
	}
	seed[0].lines = []line{
		{false, "JB Bouhier", 31 * time.Minute, "Big news. Someone moved in with me."},
		{true, "", 30 * time.Minute, "Since when??"},
		{true, "", 4 * time.Minute, "He's tiny."},
		{false, "JB Bouhier", 3 * time.Minute, "He's 3 months old and already owns the couch."},
		{true, "", 2 * time.Minute, "Who pays rent?"},
		{false, "JB Bouhier", 1 * time.Minute, "Not him. Dinner at 7 is on me, apparently 🐟"},
	}
	seed[1].lines = []line{
		{false, "Haruto Sato", 50 * time.Minute, "Are you free tonight?"},
		{false, "Haruto Sato", 40 * time.Minute, "Found a new ramen place 🍜"},
	}
	seed[2].lines = []line{
		{false, "Lukas Weber", 50 * time.Hour, "Kannst du mir das Dokument schicken?"},
		{true, "", 49 * time.Hour, "Sending it tonight"},
	}
	seed[3].lines = []line{
		{true, "", 6 * 24 * time.Hour, "Thanks for the book tip"},
		{false, "Oliver Bennett", 6 * 24 * time.Hour, "Anytime! Tell me what you think"},
	}
	seed[4].lines = []line{
		{false, "Mei Tanaka", 8 * 24 * time.Hour, "Sent you the photos from Kyoto 📸"},
	}
	seed[5].lines = []line{
		{false, "Lucía Fernández", 12 * 24 * time.Hour, "¿Nos vemos en Madrid el mes que viene?"},
		{true, "", 12 * 24 * time.Hour, "¡Claro que sí!"},
	}
	for _, s := range seed {
		var msgs []Message
		for _, l := range s.lines {
			d.nextID++
			m := Message{
				ID: fmt.Sprint(d.nextID), ConversationID: s.id,
				Timestamp: now.Add(-l.ago).UnixMilli(), FromMe: l.me, Text: l.t, Status: "received",
			}
			if l.me {
				m.Status = "read"
			} else {
				m.Sender = l.who
			}
			msgs = append(msgs, m)
		}
		last := msgs[len(msgs)-1]
		d.msgs[s.id] = msgs
		sim := int32(1)
		d.convs[s.id] = &Conversation{
			ID: s.id, Name: s.name, Color: s.color, IsGroup: s.group, IsRCS: true, AvatarID: demoAvatarID(s.id, s.group),
			SIMNumber: &sim,
			Snippet:   last.Text, SnippetMe: last.FromMe, Timestamp: last.Timestamp,
			Unread: s.id == "1" || s.id == "2", LatestMsgID: last.ID,
		}
	}
	jb := d.msgs["1"]
	jb[3].Reactions = []Reaction{{Emoji: "😂", Count: 1, Mine: true}}
	// One photo from JB, after "Since when??", for the media viewer.
	d.nextID++
	photo := Message{
		ID: fmt.Sprint(d.nextID), ConversationID: "1", Sender: "JB Bouhier", Status: "received",
		Timestamp: now.Add(-29 * time.Minute).UnixMilli(), Text: "Since 8:14 this morning. Meet Kevin 🐾",
		Media: []Media{{ID: "demo-photo-1", ThumbID: "demo-photo-1-thumb", Mime: "image/jpeg", Name: "photo.jpg", Width: 1000, Height: 1000}},
	}
	d.msgs["1"] = append(jb[:2:2], append([]Message{photo}, jb[2:]...)...)
	// Two photos from Mei, for walking the viewer.
	for i, p := range []struct{ w, h int64 }{{1500, 1000}, {1600, 1200}} {
		d.nextID++
		id := fmt.Sprintf("demo-photo-%d", i+2)
		d.msgs["5"] = append(d.msgs["5"][:i:i], append([]Message{{
			ID: fmt.Sprint(d.nextID), ConversationID: "5", Sender: "Mei Tanaka", Status: "received",
			Timestamp: now.Add(-8*24*time.Hour - time.Duration(2-i)*time.Minute).UnixMilli(),
			Media:     []Media{{ID: id, ThumbID: id + "-thumb", Mime: "image/jpeg", Name: id + ".jpg", Width: p.w, Height: p.h}},
		}}, d.msgs["5"][i:]...)...)
	}
	// A long history for Oliver, to exercise paging and scrolling.
	lines := []string{"Did you see the game last night?", "Lunch tomorrow?", "Running late, 10 min",
		"Haha that's amazing 😂", "Can you send me the address?", "Sure, here it is: 42 Main St",
		"Thanks! See you there 👍", "How was the trip?", "Long story, tell you later",
		"This one is a longer message to check that wrapping works across several lines when the bubble gets narrow."}
	var hist []Message
	for i := 0; i < 320; i++ {
		d.nextID++
		me := i%3 == 0
		m := Message{
			ID: fmt.Sprint(d.nextID), ConversationID: "4", FromMe: me, Status: "received",
			Timestamp: now.Add(-time.Duration(7*24*60+(320-i)*47) * time.Minute).UnixMilli(),
			Text:      fmt.Sprintf("#%d %s", i+1, lines[i%len(lines)]),
		}
		if me {
			m.Status = "read"
		} else {
			m.Sender = "Oliver Bennett"
		}
		hist = append(hist, m)
	}
	d.msgs["4"] = append(hist, d.msgs["4"]...)
	d.setState(stateConnected)
}

func (d *demo) handle(ctx context.Context, cmd string, raw json.RawMessage) (any, error) {
	var args struct {
		ConversationID string `json:"conversation_id"`
		Text           string `json:"text"`
		TmpID          string `json:"tmp_id"`
		ReplyTo        string `json:"reply_to"`
		MessageID      string `json:"message_id"`
		Count          int    `json:"count"`
		BeforeTS       int64  `json:"before_ts"`
	}
	_ = json.Unmarshal(raw, &args)
	switch cmd {
	case "state":
		return map[string]string{"state": d.state, "account": "demo@gmail.com"}, nil
	case "check_cookies":
		return map[string]string{"account": "demo@gmail.com"}, nil
	case "contacts":
		return []map[string]any{
			{"name": "Oliver Bennett", "number": "+15551230101", "formatted_number": "+1 555 123 0101", "top": true, "avatar_id": "p4"},
			{"name": "JB Bouhier", "number": "+15551230102", "formatted_number": "+1 555 123 0102", "top": true, "avatar_id": "p1"},
			{"name": "Alex Morgan", "number": "+15551230103", "formatted_number": "+1 555 123 0103", "top": false, "avatar_id": "palex"},
		}, nil
	case "sims":
		return []map[string]any{
			{"number": 1, "label": "Personal", "phone": "+1 555 123 0001"},
			{"number": 2, "label": "Work", "phone": "+1 555 123 0002"},
		}, nil
	case "pair_google":
		d.setState(statePairing)
		d.o.event("pair_emoji", map[string]string{"emoji": "🦊", "account": "demo@gmail.com"})
		go func() {
			time.Sleep(4 * time.Second)
			d.o.event("paired", map[string]string{"account": "demo"})
			d.setState(stateConnected)
		}()
		return nil, nil
	case "avatars":
		var args2 struct {
			IDs []string `json:"ids"`
		}
		_ = json.Unmarshal(raw, &args2)
		return d.avatars(args2.IDs)
	case "new_conversation":
		var a struct {
			Number string `json:"number"`
		}
		_ = json.Unmarshal(raw, &a)
		if a.Number == "" {
			return nil, errors.New("no phone number")
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		for _, c := range d.convs {
			if c.Name == a.Number {
				return *c, nil
			}
		}
		id := "new-" + a.Number
		sim := int32(1)
		c := &Conversation{ID: id, Name: a.Number, Color: "#5f6368", IsRCS: true, SIMNumber: &sim, Timestamp: time.Now().UnixMilli()}
		d.convs[id] = c
		return *c, nil
	case "cancel_pair", "mark_read", "typing", "react":
		return nil, nil
	case "delete_message":
		d.mu.Lock()
		defer d.mu.Unlock()
		list := d.msgs[args.ConversationID]
		for i, m := range list {
			if m.ID == args.MessageID {
				d.msgs[args.ConversationID] = append(list[:i:i], list[i+1:]...)
				return nil, nil
			}
		}
		return nil, errors.New("no such message")
	case "update_conversation":
		var a struct {
			ID     string `json:"conversation_id"`
			Action string `json:"action"`
		}
		_ = json.Unmarshal(raw, &a)
		d.mu.Lock()
		c := d.convs[a.ID]
		if c == nil {
			d.mu.Unlock()
			return nil, errors.New("no such conversation")
		}
		switch a.Action {
		case "archive", "unarchive":
			c.Archived = a.Action == "archive"
		case "delete", "block_spam":
			c.Removed = true
			delete(d.convs, a.ID)
		}
		out := *c
		d.mu.Unlock()
		d.o.event("conversation", out)
		return nil, nil
	case "logout":
		d.setState(stateUnpaired)
		return nil, nil
	case "conversations":
		d.mu.Lock()
		defer d.mu.Unlock()
		list := []Conversation{}
		for _, c := range d.convs {
			list = append(list, *c)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Timestamp > list[j].Timestamp })
		return list, nil
	case "messages":
		d.mu.Lock()
		defer d.mu.Unlock()
		var msgs []Message
		for _, m := range d.msgs[args.ConversationID] {
			if args.BeforeTS == 0 || m.Timestamp < args.BeforeTS {
				msgs = append(msgs, m)
			}
		}
		// Newest first, one page, like the phone.
		sort.Slice(msgs, func(i, j int) bool { return msgs[i].Timestamp > msgs[j].Timestamp })
		if args.Count == 0 {
			args.Count = 40
		}
		more := len(msgs) > args.Count
		if more {
			msgs = msgs[:args.Count]
		}
		time.Sleep(300 * time.Millisecond)
		return map[string]any{"messages": msgs, "more": more}, nil
	case "send":
		d.mu.Lock()
		conv := d.convs[args.ConversationID]
		if conv == nil {
			d.mu.Unlock()
			return nil, errors.New("no such conversation")
		}
		d.nextID++
		m := Message{
			ID: fmt.Sprint(d.nextID), TmpID: args.TmpID, ConversationID: conv.ID,
			Timestamp: time.Now().UnixMilli(), FromMe: true, Text: args.Text, Status: "sent",
			ReplyTo: args.ReplyTo,
		}
		d.msgs[conv.ID] = append(d.msgs[conv.ID], m)
		conv.Snippet, conv.SnippetMe, conv.Timestamp = m.Text, true, m.Timestamp
		c := *conv
		d.mu.Unlock()
		d.o.event("message", m)
		d.o.event("conversation", c)
		go d.reply(c, m)
		return map[string]string{"tmp_id": args.TmpID}, nil
	case "download":
		var a struct {
			MediaID string `json:"media_id"`
		}
		_ = json.Unmarshal(raw, &a)
		return d.photo(a.MediaID)
	}
	return nil, fmt.Errorf("unknown command %q", cmd)
}

// reply simulates delivery, a read receipt, typing, and an answer.
func (d *demo) reply(c Conversation, sent Message) {
	time.Sleep(700 * time.Millisecond)
	sent.Status = "delivered"
	d.o.event("message", sent)
	time.Sleep(900 * time.Millisecond)
	sent.Status = "read"
	d.o.event("message", sent)
	d.o.event("typing", map[string]any{"conversation_id": c.ID, "typing": true})
	time.Sleep(1800 * time.Millisecond)
	d.o.event("typing", map[string]any{"conversation_id": c.ID, "typing": false})
	d.mu.Lock()
	d.nextID++
	who := c.Name
	if c.IsGroup {
		who = "Jordan"
	}
	m := Message{
		ID: fmt.Sprint(d.nextID), ConversationID: c.ID, Timestamp: time.Now().UnixMilli(),
		Sender: who, Text: "Sounds good 👍", Status: "received",
	}
	d.msgs[c.ID] = append(d.msgs[c.ID], m)
	conv := d.convs[c.ID]
	conv.Snippet, conv.SnippetMe, conv.Timestamp, conv.Unread = m.Text, false, m.Timestamp, true
	cc := *conv
	d.mu.Unlock()
	d.o.event("message", m)
	d.o.event("conversation", cc)
}

// Everyone has a photo; a group would show its icon.
func demoAvatarID(id string, group bool) string {
	if group {
		return ""
	}
	return "p" + id
}

// avatars copies each demo contact's photo into the media folder. IDs
// without one get initials.
func (d *demo) avatars(ids []string) (any, error) {
	out := map[string]string{}
	for _, id := range ids {
		path, err := d.asset("avatar-"+id+".jpg", "demo_"+id+".jpg")
		if err != nil {
			out[id] = ""
			continue
		}
		out[id] = path
	}
	return out, nil
}

// photo serves the demo pictures and their thumbnails.
func (d *demo) photo(id string) (any, error) {
	name, ok := strings.CutPrefix(id, "demo-")
	if !ok {
		return nil, errors.New("no such media")
	}
	path, err := d.asset(name+".jpg", id+".jpg")
	if err != nil {
		return nil, errors.New("no such media")
	}
	return map[string]string{"path": path}, nil
}

// asset writes an embedded demo file to the media folder once.
func (d *demo) asset(name, as string) (string, error) {
	path := filepath.Join(d.dir, "media", as)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	data, err := demoAssets.ReadFile("assets/demo/" + name)
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o600)
}
