package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

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
		{"1", "Maya Chen", "#e8710a", false, nil},
		{"2", "Climbing crew", "#1a73e8", true, nil},
		{"3", "Dad", "#188038", false, nil},
		{"4", "Sam Rivera", "#a142f4", false, nil},
		{"5", "Bank of Example", "#5f6368", false, nil},
		{"6", "Léa Dubois", "#d93025", false, nil},
	}
	type line = struct {
		me  bool
		who string
		ago time.Duration
		t   string
	}
	seed[0].lines = []line{
		{false, "Maya Chen", 26 * time.Hour, "Did you end up booking the cabin?"},
		{true, "", 25 * time.Hour, "Yes! Friday to Sunday 🏔️"},
		{false, "Maya Chen", 25 * time.Hour, "Amazing. I'll bring the board games"},
		{false, "Maya Chen", 4 * time.Minute, "Leaving work now, see you at 7?"},
	}
	seed[1].lines = []line{
		{false, "Jordan", 3 * time.Hour, "Gym at 6 tomorrow?"},
		{false, "Priya", 3 * time.Hour, "I'm in"},
		{true, "", 2 * time.Hour, "Count me in too"},
		{false, "Jordan", 40 * time.Minute, "Bring chalk, I'm out"},
	}
	seed[2].lines = []line{
		{false, "Dad", 50 * time.Hour, "Call me when you get a chance"},
		{true, "", 49 * time.Hour, "Will do tonight"},
	}
	seed[3].lines = []line{
		{true, "", 6 * 24 * time.Hour, "Thanks for the book recommendation"},
		{false, "Sam Rivera", 6 * 24 * time.Hour, "Anytime! Let me know what you think"},
	}
	seed[4].lines = []line{
		{false, "Bank of Example", 8 * 24 * time.Hour, "Your verification code is 482913. Don't share it with anyone."},
	}
	seed[5].lines = []line{
		{false, "Léa Dubois", 12 * 24 * time.Hour, "On se voit à Paris le mois prochain ?"},
		{true, "", 12 * 24 * time.Hour, "Oui, avec plaisir !"},
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
	d.msgs["1"][len(d.msgs["1"])-2].Reactions = []Reaction{{Emoji: "😂", Count: 1, Mine: true}}
	// Two photos from Maya, for the media viewer.
	for i, caption := range []string{"Look at this view from the cabin 🏔️", ""} {
		d.nextID++
		id := fmt.Sprintf("demo-photo-%d", i+1)
		d.msgs["1"] = append(d.msgs["1"], Message{
			ID: fmt.Sprint(d.nextID), ConversationID: "1", Sender: "Maya Chen", Status: "received",
			Timestamp: now.Add(-time.Duration(3-i) * time.Minute).UnixMilli(), Text: caption,
			Media: []Media{{ID: id, ThumbID: id + "-thumb", Mime: "image/jpeg", Name: id + ".jpg", Width: 1600, Height: 1000}},
		})
	}
	// A long history for Sam, to exercise paging and scrolling.
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
			m.Sender = "Sam Rivera"
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
			{"name": "Sam Rivera", "number": "+15551230101", "formatted_number": "+1 555 123 0101", "top": true, "avatar_id": "p4"},
			{"name": "Maya Chen", "number": "+15551230102", "formatted_number": "+1 555 123 0102", "top": true, "avatar_id": "p1"},
			{"name": "Alex Morgan", "number": "+15551230103", "formatted_number": "+1 555 123 0103", "top": false, "avatar_id": ""},
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

// Maya and Sam have "photos"; everyone else shows initials.
func demoAvatarID(id string, group bool) string {
	if group || (id != "1" && id != "4") {
		return ""
	}
	return "p" + id
}

// avatars draws a soft two-tone portrait stand-in for each demo ID.
func (d *demo) avatars(ids []string) (any, error) {
	out := map[string]string{}
	for _, id := range ids {
		path := filepath.Join(d.dir, "media", "demo_"+id+".png")
		if _, err := os.Stat(path); err != nil {
			img := image.NewRGBA(image.Rect(0, 0, 96, 96))
			for y := 0; y < 96; y++ {
				for x := 0; x < 96; x++ {
					dx, dy := float64(x-48), float64(y-38)
					c := color.RGBA{uint8(90 + y), uint8(140 + x/2), 200, 255}
					if id == "p4" {
						c = color.RGBA{220, uint8(120 + y/2), uint8(80 + x), 255}
					}
					if dx*dx+dy*dy < 18*18 || (y > 64 && dx*dx < float64((y-40)*(y-40))) {
						c = color.RGBA{250, 235, 220, 255} // head and shoulders
					}
					img.Set(x, y, c)
				}
			}
			f, err := os.Create(path)
			if err != nil {
				return nil, err
			}
			_ = png.Encode(f, img)
			f.Close()
		}
		out[id] = path
	}
	return out, nil
}

// photo draws a demo landscape: sky, sun and hills. Thumbnails are small.
func (d *demo) photo(id string) (any, error) {
	if !strings.HasPrefix(id, "demo-photo-") {
		return nil, errors.New("no such media")
	}
	path := filepath.Join(d.dir, "media", id+".jpg")
	if _, err := os.Stat(path); err != nil {
		w, h := 1600, 1000
		if strings.HasSuffix(id, "-thumb") {
			w, h = 320, 200
		}
		second := strings.Contains(id, "-2")
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				fy := float64(y) / float64(h)
				fx := float64(x) / float64(w)
				c := color.RGBA{uint8(40 + 120*fy), uint8(110 + 90*fy), uint8(230 - 60*fy), 255}
				if second {
					c = color.RGBA{uint8(250 - 40*fy), uint8(140 + 60*fy), uint8(90 + 40*fy), 255}
				}
				sx, sy := fx-0.72, fy-0.28
				if sx*sx+sy*sy*2.5 < 0.006 {
					c = color.RGBA{255, 236, 170, 255}
				}
				hill := 0.62 + 0.08*math.Sin(fx*9) + 0.05*math.Sin(fx*23+1)
				if fy > hill {
					c = color.RGBA{uint8(30 + 40*fx), uint8(90 + 50*fy), 60, 255}
				}
				img.Set(x, y, c)
			}
		}
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		_ = jpeg.Encode(f, img, &jpeg.Options{Quality: 88})
		f.Close()
	}
	return map[string]string{"path": path}, nil
}
