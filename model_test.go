package main

import (
	"testing"

	"go.mau.fi/mautrix-gmessages/pkg/libgm/gmproto"
)

func participant(id, number, name string, me, visible bool) *gmproto.Participant {
	return &gmproto.Participant{
		ID:        &gmproto.SmallInfo{ParticipantID: id, Number: number},
		FullName:  name,
		IsMe:      me,
		IsVisible: visible,
	}
}

// Regression: our own photo showed on many contacts because our own entry
// isn't always flagged IsMe, and hidden entries were taken for the contact.
func TestOneToOneAvatarIsTheOtherPerson(t *testing.T) {
	cases := []struct {
		name string
		conv *gmproto.Conversation
		want string
	}{
		{
			name: "self flagged IsMe",
			conv: &gmproto.Conversation{Participants: []*gmproto.Participant{
				participant("me", "+15550001", "Me", true, true),
				participant("them", "+15550002", "Sam", false, true),
			}},
			want: "them",
		},
		{
			name: "self not flagged, but it is the outgoing ID",
			conv: &gmproto.Conversation{
				DefaultOutgoingID: "me",
				Participants: []*gmproto.Participant{
					participant("them", "+15550002", "Sam", false, true),
					participant("me", "+15550001", "Me", false, true),
				},
			},
			want: "them",
		},
		{
			name: "hidden and number-less entries are skipped",
			conv: &gmproto.Conversation{Participants: []*gmproto.Participant{
				participant("ghost", "+15550003", "", false, false),
				participant("blank", "", "", false, true),
				participant("them", "+15550002", "Sam", false, true),
			}},
			want: "them",
		},
		{
			name: "groups have no single avatar",
			conv: &gmproto.Conversation{IsGroupChat: true, Participants: []*gmproto.Participant{
				participant("a", "+15550002", "A", false, true),
				participant("b", "+15550004", "B", false, true),
			}},
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := convertConversation(c.conv).AvatarID; got != c.want {
				t.Errorf("AvatarID = %q, want %q", got, c.want)
			}
		})
	}
}

// Regression: JPEGs were saved as ".pjpeg", which the image decoder skips.
func TestExtensionForCommonTypes(t *testing.T) {
	cases := map[string]string{
		"image/jpeg":               ".jpg",
		"image/jpg":                ".jpg",
		"IMAGE/JPEG; charset=x":    ".jpg",
		"image/png":                ".png",
		"image/webp":               ".webp",
		"image/heic":               ".heic",
		"video/mp4":                ".mp4",
		"application/pdf":          ".pdf",
		"application/x-not-a-type": ".bin",
	}
	for mime, want := range cases {
		if got := extensionFor(mime); got != want {
			t.Errorf("extensionFor(%q) = %q, want %q", mime, got, want)
		}
	}
}

func TestStatusOf(t *testing.T) {
	cases := map[gmproto.MessageStatusType]string{
		gmproto.MessageStatusType_OUTGOING_SENDING:             "sending",
		gmproto.MessageStatusType_OUTGOING_COMPLETE:            "sent",
		gmproto.MessageStatusType_OUTGOING_DELIVERED:           "delivered",
		gmproto.MessageStatusType_OUTGOING_DISPLAYED:           "read",
		gmproto.MessageStatusType_OUTGOING_FAILED_GENERIC:      "failed",
		gmproto.MessageStatusType_INCOMING_COMPLETE:            "received",
		gmproto.MessageStatusType_TOMBSTONE_PARTICIPANT_JOINED: "system",
	}
	for status, want := range cases {
		if got := statusOf(status); got != want {
			t.Errorf("statusOf(%v) = %q, want %q", status, got, want)
		}
	}
}
