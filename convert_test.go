package main

import (
	"testing"

	"go.mau.fi/mautrix-gmessages/pkg/libgm/gmproto"
)

// What the UI shows for messages and conversations.

func TestConvertIncomingMessage(t *testing.T) {
	parts := map[string]*gmproto.Participant{
		"me":  participant("me", "+15550001", "Me", true, true),
		"sam": participant("sam", "+15550002", "Sam", false, true),
	}
	parts["sam"].AvatarHexColor = "#ff0000"
	m := textMessage("m1", "c1", "sam", "first line")
	m.MessageInfo = append(m.MessageInfo,
		&gmproto.MessageInfo{Data: &gmproto.MessageInfo_MessageContent{MessageContent: &gmproto.MessageContent{Content: "second line"}}},
		&gmproto.MessageInfo{Data: &gmproto.MessageInfo_MediaContent{MediaContent: &gmproto.MediaContent{
			MediaID: "media1", DecryptionKey: []byte{1, 2, 3}, MimeType: "image/jpeg", MediaName: "a.jpg",
			Size: 1234, Dimensions: &gmproto.Dimensions{Width: 640, Height: 480},
		}}},
	)
	m.ReplyMessage = &gmproto.ReplyMessage{MessageID: "m0"}
	m.Reactions = []*gmproto.ReactionEntry{
		{Data: &gmproto.ReactionData{Unicode: "👍"}, ParticipantIDs: []string{"me", "sam"}},
		{Data: &gmproto.ReactionData{Unicode: "😂"}, ParticipantIDs: []string{"sam"}},
		{Data: &gmproto.ReactionData{}, ParticipantIDs: []string{"sam"}}, // custom emoji: skipped
	}

	got := convertMessage(m, parts)

	if got.ID != "m1" || got.ConversationID != "c1" || got.FromMe || got.Status != "received" {
		t.Fatalf("basics %+v", got)
	}
	if got.Timestamp != 1_700_000_000_000 {
		t.Errorf("timestamp %d, want millis", got.Timestamp)
	}
	if got.Sender != "Sam" || got.SenderColor != "#ff0000" || got.SenderID != "sam" {
		t.Errorf("sender %q %q %q", got.Sender, got.SenderColor, got.SenderID)
	}
	if got.Text != "first line\nsecond line" {
		t.Errorf("text %q", got.Text)
	}
	if got.ReplyTo != "m0" {
		t.Errorf("reply to %q", got.ReplyTo)
	}
	if len(got.Media) != 1 || got.Media[0].Key != "AQID" || got.Media[0].Width != 640 || got.Media[0].Mime != "image/jpeg" {
		t.Errorf("media %+v", got.Media)
	}
	if len(got.Reactions) != 2 ||
		got.Reactions[0] != (Reaction{Emoji: "👍", Count: 2, Mine: true}) ||
		got.Reactions[1] != (Reaction{Emoji: "😂", Count: 1, Mine: false}) {
		t.Errorf("reactions %+v", got.Reactions)
	}
}

func TestConvertOutgoingMessage(t *testing.T) {
	m := textMessage("m2", "c1", "me", "on my way")
	m.MessageStatus.Status = gmproto.MessageStatusType_OUTGOING_DELIVERED
	m.TmpID = "tmp_1"
	got := convertMessage(m, nil)
	if !got.FromMe || got.Status != "delivered" || got.TmpID != "tmp_1" {
		t.Fatalf("%+v", got)
	}
}

func TestSenderFallbacks(t *testing.T) {
	t.Run("number when the contact has no name", func(t *testing.T) {
		p := participant("x", "+15550003", "", false, true)
		p.FormattedNumber = "(555) 000-0003"
		got := convertMessage(textMessage("m", "c", "x", "hi"), map[string]*gmproto.Participant{"x": p})
		if got.Sender != "(555) 000-0003" {
			t.Fatalf("sender %q", got.Sender)
		}
	})
	t.Run("sender carried on the message", func(t *testing.T) {
		m := textMessage("m", "c", "x", "hi")
		m.SenderParticipant = &gmproto.Participant{FullName: "Alex", AvatarHexColor: "#00ff00"}
		got := convertMessage(m, nil)
		if got.Sender != "Alex" || got.SenderColor != "#00ff00" {
			t.Fatalf("sender %q %q", got.Sender, got.SenderColor)
		}
	})
}

func TestSystemMessageGetsText(t *testing.T) {
	m := &gmproto.Message{MessageID: "s1", MessageStatus: &gmproto.MessageStatus{Status: gmproto.MessageStatusType_TOMBSTONE_PARTICIPANT_JOINED}}
	got := convertMessage(m, nil)
	if got.Status != "system" || got.Text != "Someone joined" {
		t.Fatalf("%+v", got)
	}
}

func TestConvertConversation(t *testing.T) {
	group := &gmproto.Conversation{
		ConversationID:       "g1",
		Name:                 "Family",
		IsGroupChat:          true,
		Unread:               true,
		LastMessageTimestamp: 1_700_000_000_000_000,
		Type:                 gmproto.ConversationType_RCS,
		LatestMessage:        &gmproto.LatestMessage{DisplayContent: "dinner?", DisplayName: "Sam"},
		Participants: []*gmproto.Participant{
			participant("me", "+15550001", "Me", true, true),
			participant("sam", "+15550002", "Sam", false, true),
			participant("ana", "+15550004", "Ana", false, true),
		},
		SimCard: &gmproto.SIMCard{SIMData: &gmproto.SIMData{SIMPayload: &gmproto.SIMPayload{SIMNumber: 2}}},
	}
	got := convertConversation(group)
	if got.Snippet != "Sam: dinner?" || got.SnippetMe {
		t.Errorf("snippet %q fromMe=%v", got.Snippet, got.SnippetMe)
	}
	if !got.IsGroup || !got.Unread || !got.IsRCS || got.Timestamp != 1_700_000_000_000 {
		t.Errorf("flags %+v", got)
	}
	if len(got.Members) != 2 || got.AvatarID != "" {
		t.Errorf("members %v avatar %q (groups have no one-to-one avatar)", got.Members, got.AvatarID)
	}
	if got.SIMNumber == nil || *got.SIMNumber != 2 {
		t.Errorf("sim %v", got.SIMNumber)
	}

	// Our own last message: no name prefix.
	group.LatestMessage.FromMe = 1
	if got := convertConversation(group); got.Snippet != "dinner?" || !got.SnippetMe {
		t.Errorf("own snippet %q fromMe=%v", got.Snippet, got.SnippetMe)
	}
}

func TestConversationFolders(t *testing.T) {
	cases := []struct {
		status            gmproto.ConversationStatus
		archived, removed bool
	}{
		{gmproto.ConversationStatus_ACTIVE, false, false},
		{gmproto.ConversationStatus_ARCHIVED, true, false},
		{gmproto.ConversationStatus_DELETED, false, true},
		{gmproto.ConversationStatus_SPAM_FOLDER, false, true},
		{gmproto.ConversationStatus_BLOCKED_FOLDER, false, true},
		{gmproto.ConversationStatus_TRASH_FOLDER, false, true},
	}
	for _, tc := range cases {
		got := convertConversation(&gmproto.Conversation{ConversationID: "c", Status: tc.status})
		if got.Archived != tc.archived || got.Removed != tc.removed {
			t.Errorf("%s: archived=%v removed=%v", tc.status, got.Archived, got.Removed)
		}
	}
}
