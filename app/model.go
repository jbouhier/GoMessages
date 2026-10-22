package main

import (
	"encoding/base64"
	"strings"

	"go.mau.fi/mautrix-gmessages/pkg/libgm/gmproto"
)

// The app never sees protobuf: these are the shapes it receives.

type Conversation struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Snippet     string   `json:"snippet"`
	SnippetMe   bool     `json:"snippet_from_me"`
	Timestamp   int64    `json:"timestamp"` // unix millis
	Unread      bool     `json:"unread"`
	IsGroup     bool     `json:"is_group"`
	Pinned      bool     `json:"pinned"`
	Archived    bool     `json:"archived"`
	Removed     bool     `json:"removed,omitempty"` // deleted, trashed, spam or blocked: the list drops it
	Color       string   `json:"color"`
	ReadOnly    bool     `json:"read_only"`
	IsRCS       bool     `json:"is_rcs"`
	LatestMsgID string   `json:"latest_message_id"`
	Members     []string `json:"members"`
	// AvatarID is the other participant of a one-to-one chat, for its photo.
	AvatarID  string `json:"avatar_id,omitempty"`
	SIMNumber *int32 `json:"sim_number,omitempty"`
	// People are the other participants, for the info panel.
	People []Person `json:"people,omitempty"`
}

// Person is one participant as the phone describes them. The phone sends
// no email, address or other numbers: only what's here.
type Person struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Number    string `json:"number"`           // E.164
	Formatted string `json:"formatted_number"` // as the phone shows it
	Color     string `json:"color,omitempty"`
	// Saved is false for a number that isn't in the phone's contacts.
	Saved bool `json:"saved"`
}

type Media struct {
	ID       string `json:"id"`
	Key      string `json:"key"` // base64
	ThumbID  string `json:"thumb_id,omitempty"`
	ThumbKey string `json:"thumb_key,omitempty"`
	Mime     string `json:"mime"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Width    int64  `json:"width,omitempty"`
	Height   int64  `json:"height,omitempty"`
}

type Reaction struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
	Mine  bool   `json:"mine"`
}

type Message struct {
	ID             string     `json:"id"`
	TmpID          string     `json:"tmp_id,omitempty"`
	ConversationID string     `json:"conversation_id"`
	Timestamp      int64      `json:"timestamp"` // unix millis
	FromMe         bool       `json:"from_me"`
	Sender         string     `json:"sender"`
	SenderColor    string     `json:"sender_color,omitempty"`
	SenderID       string     `json:"sender_id,omitempty"`
	Text           string     `json:"text"`
	Media          []Media    `json:"media,omitempty"`
	Status         string     `json:"status"` // sending, sent, delivered, read, failed, received, system, deleted
	ReplyTo        string     `json:"reply_to,omitempty"`
	Reactions      []Reaction `json:"reactions,omitempty"`
	IsOld          bool       `json:"is_old,omitempty"`
}

func convertConversation(c *gmproto.Conversation) Conversation {
	var members []string
	var people []Person
	var avatarID string
	self := c.GetDefaultOutgoingID()
	for _, p := range c.GetParticipants() {
		id := p.GetID().GetParticipantID()
		// Our own entry isn't always flagged IsMe; the outgoing ID is ours too.
		// Hidden entries and ones without a number aren't the other person
		// (the bridge applies the same rule).
		if p.GetIsMe() || id == self || !p.GetIsVisible() || p.GetID().GetNumber() == "" {
			continue
		}
		if !c.GetIsGroupChat() && avatarID == "" {
			avatarID = id
		}
		formatted := p.GetFormattedNumber()
		if formatted == "" {
			formatted = p.GetID().GetNumber()
		}
		name := p.GetFullName()
		if name == "" {
			name = formatted
		}
		members = append(members, name)
		people = append(people, Person{
			ID: id, Name: name, Number: p.GetID().GetNumber(), Formatted: formatted,
			Color: p.GetAvatarHexColor(), Saved: p.GetContactID() != "",
		})
	}
	latest := c.GetLatestMessage()
	snippet := latest.GetDisplayContent()
	if c.GetIsGroupChat() && latest.GetFromMe() == 0 && latest.GetDisplayName() != "" {
		snippet = latest.GetDisplayName() + ": " + snippet
	}
	var simNumber *int32
	if payload := c.GetSimCard().GetSIMData().GetSIMPayload(); payload != nil {
		number := payload.GetSIMNumber()
		simNumber = &number
	}
	return Conversation{
		ID:          c.GetConversationID(),
		Name:        c.GetName(),
		Snippet:     snippet,
		SnippetMe:   latest.GetFromMe() != 0,
		Timestamp:   c.GetLastMessageTimestamp() / 1000,
		Unread:      c.GetUnread(),
		IsGroup:     c.GetIsGroupChat(),
		Pinned:      c.GetPinned(),
		Archived:    c.GetStatus() == gmproto.ConversationStatus_ARCHIVED,
		Removed:     removedStatus(c.GetStatus()),
		Color:       c.GetAvatarHexColor(),
		ReadOnly:    c.GetReadOnly(),
		IsRCS:       c.GetType() == gmproto.ConversationType_RCS,
		LatestMsgID: c.GetLatestMessageID(),
		Members:     members,
		AvatarID:    avatarID,
		SIMNumber:   simNumber,
		People:      people,
	}
}

func removedStatus(s gmproto.ConversationStatus) bool {
	switch s {
	case gmproto.ConversationStatus_DELETED, gmproto.ConversationStatus_SPAM_FOLDER,
		gmproto.ConversationStatus_BLOCKED_FOLDER, gmproto.ConversationStatus_TRASH_FOLDER:
		return true
	}
	return false
}

func statusOf(s gmproto.MessageStatusType) string {
	if s == gmproto.MessageStatusType_MESSAGE_DELETED {
		return "deleted" // the UI drops it from the thread
	}
	switch {
	case s >= 200:
		return "system"
	case s >= 100:
		return "received"
	}
	switch s {
	case gmproto.MessageStatusType_OUTGOING_DELIVERED:
		return "delivered"
	case gmproto.MessageStatusType_OUTGOING_DISPLAYED:
		return "read"
	case gmproto.MessageStatusType_OUTGOING_COMPLETE:
		return "sent"
	case gmproto.MessageStatusType_OUTGOING_YET_TO_SEND,
		gmproto.MessageStatusType_OUTGOING_SENDING,
		gmproto.MessageStatusType_OUTGOING_RESENDING,
		gmproto.MessageStatusType_OUTGOING_AWAITING_RETRY,
		gmproto.MessageStatusType_OUTGOING_SEND_AFTER_PROCESSING,
		gmproto.MessageStatusType_OUTGOING_VALIDATING,
		gmproto.MessageStatusType_OUTGOING_NOT_DELIVERED_YET,
		gmproto.MessageStatusType_OUTGOING_SCHEDULED:
		return "sending"
	case gmproto.MessageStatusType_OUTGOING_DRAFT:
		return "draft"
	}
	return "failed"
}

func b64(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// convertMessage turns a protobuf message into the app's shape. participants
// maps participant IDs to display names/colors from the conversation.
func convertMessage(m *gmproto.Message, participants map[string]*gmproto.Participant) Message {
	status := m.GetMessageStatus().GetStatus()
	msg := Message{
		ID:             m.GetMessageID(),
		TmpID:          m.GetTmpID(),
		ConversationID: m.GetConversationID(),
		Timestamp:      m.GetTimestamp() / 1000,
		FromMe:         status < 100,
		Status:         statusOf(status),
		ReplyTo:        m.GetReplyMessage().GetMessageID(),
		SenderID:       m.GetParticipantID(),
	}
	if p := participants[m.GetParticipantID()]; p != nil {
		msg.Sender = p.GetFullName()
		if msg.Sender == "" {
			msg.Sender = p.GetFormattedNumber()
		}
		msg.SenderColor = p.GetAvatarHexColor()
	} else if sp := m.GetSenderParticipant(); sp != nil {
		msg.Sender = sp.GetFullName()
		msg.SenderColor = sp.GetAvatarHexColor()
	}
	var text []string
	for _, info := range m.GetMessageInfo() {
		if mc := info.GetMessageContent(); mc != nil && mc.GetContent() != "" {
			text = append(text, mc.GetContent())
		}
		if md := info.GetMediaContent(); md != nil {
			msg.Media = append(msg.Media, Media{
				ID:       md.GetMediaID(),
				Key:      b64(md.GetDecryptionKey()),
				ThumbID:  md.GetThumbnailMediaID(),
				ThumbKey: b64(md.GetThumbnailDecryptionKey()),
				Mime:     md.GetMimeType(),
				Name:     md.GetMediaName(),
				Size:     md.GetSize(),
				Width:    md.GetDimensions().GetWidth(),
				Height:   md.GetDimensions().GetHeight(),
			})
		}
	}
	msg.Text = strings.Join(text, "\n")
	if msg.Status == "system" && msg.Text == "" {
		msg.Text = systemText(status)
	}
	counts := map[string]*Reaction{}
	var order []string
	for _, r := range m.GetReactions() {
		emoji := r.GetData().GetUnicode()
		if emoji == "" {
			continue
		}
		rc := counts[emoji]
		if rc == nil {
			rc = &Reaction{Emoji: emoji}
			counts[emoji] = rc
			order = append(order, emoji)
		}
		for _, pid := range r.GetParticipantIDs() {
			rc.Count++
			if p := participants[pid]; p != nil && p.GetIsMe() {
				rc.Mine = true
			}
		}
	}
	for _, e := range order {
		msg.Reactions = append(msg.Reactions, *counts[e])
	}
	return msg
}

func systemText(s gmproto.MessageStatusType) string {
	switch s {
	case gmproto.MessageStatusType_TOMBSTONE_PARTICIPANT_JOINED:
		return "Someone joined"
	case gmproto.MessageStatusType_TOMBSTONE_PARTICIPANT_LEFT:
		return "Someone left"
	case gmproto.MessageStatusType_TOMBSTONE_SELF_LEFT:
		return "You left"
	case gmproto.MessageStatusType_TOMBSTONE_RCS_GROUP_CREATED, gmproto.MessageStatusType_TOMBSTONE_MMS_GROUP_CREATED:
		return "Group created"
	case gmproto.MessageStatusType_TOMBSTONE_ONE_ON_ONE_RCS_CREATED:
		return "Chatting with RCS"
	case gmproto.MessageStatusType_TOMBSTONE_ONE_ON_ONE_SMS_CREATED:
		return "Texting with SMS/MMS"
	}
	return ""
}
