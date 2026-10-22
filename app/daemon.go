package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/util/exhttp"

	"go.mau.fi/mautrix-gmessages/pkg/libgm"
	"go.mau.fi/mautrix-gmessages/pkg/libgm/events"
	"go.mau.fi/mautrix-gmessages/pkg/libgm/gmproto"
)

// Link states the app renders.
const (
	stateUnpaired     = "unpaired"
	statePairing      = "pairing"
	stateConnecting   = "connecting"
	stateConnected    = "connected"
	statePhoneOffline = "phone_offline"
	stateLoggedOut    = "logged_out"
)

type daemon struct {
	o   *out
	dir string
	log zerolog.Logger

	mu         sync.Mutex
	client     *libgm.Client
	state      string
	account    string
	convs      map[string]*gmproto.Conversation
	settings   *gmproto.Settings
	pairCancel context.CancelFunc
	saveMu     sync.Mutex
}

func newDaemon(o *out, dir string, log zerolog.Logger) *daemon {
	return &daemon{o: o, dir: dir, log: log, convs: map[string]*gmproto.Conversation{}}
}

func (d *daemon) sessionPath() string { return filepath.Join(d.dir, "session.json") }

func (d *daemon) setState(state string) {
	d.mu.Lock()
	changed := d.state != state
	d.state = state
	account := d.account
	d.mu.Unlock()
	if changed {
		d.o.event("state", map[string]string{"state": state, "account": account})
	}
}

func (d *daemon) start(ctx context.Context) {
	ad, err := d.loadSession()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			d.log.Warn().Err(err).Msg("Unreadable session, starting unpaired")
		}
		d.setState(stateUnpaired)
		return
	}
	go d.connect(ctx, ad)
}

func (d *daemon) shutdown() {
	d.mu.Lock()
	c := d.client
	d.mu.Unlock()
	if c != nil {
		c.Disconnect()
		d.saveSession(c.AuthData)
	}
}

// ---- session ----

func (d *daemon) loadSession() (*libgm.AuthData, error) {
	data, err := os.ReadFile(d.sessionPath())
	if err != nil {
		return nil, err
	}
	ad := &libgm.AuthData{}
	if err := json.Unmarshal(data, ad); err != nil {
		return nil, err
	}
	if ad.SessionID == uuid.Nil {
		ad.SessionID = uuid.New()
	}
	return ad, nil
}

func (d *daemon) saveSession(ad *libgm.AuthData) {
	d.saveMu.Lock()
	defer d.saveMu.Unlock()
	ad.CookiesLock.RLock()
	data, err := json.Marshal(ad)
	ad.CookiesLock.RUnlock()
	if err != nil {
		d.log.Err(err).Msg("Marshal session")
		return
	}
	tmp := d.sessionPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err == nil {
		err = os.Rename(tmp, d.sessionPath())
	}
	if err != nil {
		d.log.Err(err).Msg("Save session")
	}
}

// ---- connection ----

func (d *daemon) newClient(ad *libgm.AuthData) *libgm.Client {
	return libgm.NewClient(ad, nil, d.log.With().Str("component", "libgm").Logger(), exhttp.ClientSettings{})
}

// eventQueueSize bounds the events waiting for the handler. A burst past it
// (thousands of replayed messages) is still handled, just out of order.
const eventQueueSize = 4096

// queueEvents wraps handle so the caller never waits on it. libgm calls its
// event handler on the long-poll reader, and anything that waits on the phone
// there (GetConversation for a message's sender) deadlocks: the reply arrives
// on that same reader. Events are handled in order on their own goroutine;
// when the queue is full, the extra event gets a goroutine of its own rather
// than block the reader.
func queueEvents(size int, handle func(any), log zerolog.Logger) func(any) {
	queue := make(chan any, size)
	go func() {
		for evt := range queue {
			handle(evt)
		}
	}()
	return func(evt any) {
		select {
		case queue <- evt:
		default:
			log.Warn().Type("type", evt).Msg("Event queue full, handling out of order")
			go handle(evt)
		}
	}
}

// connectBackoff is the wait before connect attempt+1. A var so tests can
// shorten it.
var connectBackoff = func(attempt int) time.Duration {
	return min(time.Duration(attempt+1)*3*time.Second, 30*time.Second)
}

func (d *daemon) connect(ctx context.Context, ad *libgm.AuthData) {
	d.setState(stateConnecting)
	c := d.newClient(ad)
	c.SetEventHandler(queueEvents(eventQueueSize, func(evt any) { d.onEvent(ctx, c, evt) }, d.log))
	d.mu.Lock()
	if old := d.client; old != nil {
		old.Disconnect()
	}
	d.client = c
	d.account = ad.Mobile.GetSourceID()
	d.mu.Unlock()

	if d.connectWithRetry(ctx, c.Connect) {
		go d.refreshAccount(ctx, c)
	}
}

// connectWithRetry calls connect until it succeeds, Google rejects the
// session, or ctx ends. It reports whether the link is up.
func (d *daemon) connectWithRetry(ctx context.Context, connect func(context.Context) error) bool {
	for attempt := 0; ; attempt++ {
		err := connect(ctx)
		if err == nil {
			// libgm has no "ready" event: connected means long polling is up.
			// A slow phone shows up later as PhoneNotResponding.
			d.log.Debug().Int("attempt", attempt).Msg("Connected")
			d.setState(stateConnected)
			return true
		}
		d.log.Err(err).Int("attempt", attempt).Msg("Connect failed")
		if strings.Contains(err.Error(), "failed to refresh auth token") && attempt >= 2 {
			d.loggedOut("Google rejected this session. Pair again.")
			return false
		}
		select {
		case <-time.After(connectBackoff(attempt)):
		case <-ctx.Done():
			return false
		}
	}
}

func (d *daemon) loggedOut(reason string) {
	d.log.Warn().Str("reason", reason).Msg("Session ended, removing it")
	d.mu.Lock()
	c := d.client
	d.client = nil
	d.convs = map[string]*gmproto.Conversation{}
	d.mu.Unlock()
	if c != nil {
		c.Disconnect()
	}
	_ = os.Remove(d.sessionPath())
	_ = os.Remove(d.contactsPath())
	d.o.event("logged_out", map[string]string{"reason": reason})
	d.setState(stateUnpaired)
}

func (d *daemon) onEvent(ctx context.Context, c *libgm.Client, evt any) {
	switch e := evt.(type) {
	case *events.ListenRecovered, *events.PhoneRespondingAgain:
		d.setState(stateConnected)
	case *events.PhoneNotResponding:
		d.setState(statePhoneOffline)
	case *events.AuthTokenRefreshed:
		d.saveSession(c.AuthData)
	case *events.GaiaLoggedOut:
		d.loggedOut("Signed out of Google.")
	case *gmproto.RevokePairData:
		d.loggedOut("Unpaired from the phone.")
	case *events.ListenFatalError:
		d.log.Err(e.Error).Msg("Fatal listen error")
		d.loggedOut("Connection rejected: " + e.Error.Error())
	case *events.ListenTemporaryError:
		d.setState(stateConnecting)
	case *gmproto.Conversation:
		d.cacheConversations([]*gmproto.Conversation{e})
		d.o.event("conversation", convertConversation(e))
	case *gmproto.Settings:
		if len(e.GetSIMCards()) > 0 {
			d.mu.Lock()
			d.settings = e
			d.mu.Unlock()
			d.o.event("sims", simChoices(e))
		}
	case *libgm.WrappedMessage:
		// Sender names need the conversation. Don't hold the queue long for
		// it: a message without names beats a stalled app.
		pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		parts := d.participants(pctx, e.GetConversationID())
		cancel()
		msg := convertMessage(e.Message, parts)
		msg.IsOld = e.IsOld
		d.o.event("message", msg)
	case *gmproto.TypingData:
		d.o.event("typing", map[string]any{
			"conversation_id": e.GetConversationID(),
			"typing":          e.GetType() == gmproto.TypingTypes_STARTED_TYPING,
		})
	default:
		d.log.Debug().Type("type", evt).Msg("Unhandled event")
	}
}

func (d *daemon) refreshAccount(ctx context.Context, c *libgm.Client) {
	if err := c.FetchConfig(ctx); err != nil {
		d.log.Debug().Err(err).Msg("Account email unavailable")
		return
	}
	email := c.Config.GetDeviceInfo().GetEmail()
	if email == "" {
		return
	}
	d.mu.Lock()
	if d.client != c {
		d.mu.Unlock()
		return
	}
	d.account = email
	state := d.state
	d.mu.Unlock()
	d.o.event("state", map[string]string{"state": state, "account": email})
}

func (d *daemon) cacheConversations(convs []*gmproto.Conversation) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range convs {
		d.convs[c.GetConversationID()] = c
	}
}

func (d *daemon) conversation(ctx context.Context, id string) (*gmproto.Conversation, error) {
	d.mu.Lock()
	conv := d.convs[id]
	c := d.client
	d.mu.Unlock()
	if conv != nil {
		return conv, nil
	}
	if c == nil {
		return nil, errNotConnected
	}
	conv, err := c.GetConversation(ctx, id)
	if err != nil {
		return nil, err
	}
	d.cacheConversations([]*gmproto.Conversation{conv})
	return conv, nil
}

func (d *daemon) participants(ctx context.Context, convID string) map[string]*gmproto.Participant {
	out := map[string]*gmproto.Participant{}
	conv, err := d.conversation(ctx, convID)
	if err != nil {
		return out
	}
	for _, p := range conv.GetParticipants() {
		out[p.GetID().GetParticipantID()] = p
	}
	return out
}

var errNotConnected = errors.New("not connected to the phone")

func (d *daemon) connected() (*libgm.Client, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.client == nil {
		return nil, errNotConnected
	}
	return d.client, nil
}

// ---- pairing ----

func (d *daemon) beginPairing() context.Context {
	d.mu.Lock()
	if d.pairCancel != nil {
		d.pairCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	d.pairCancel = cancel
	d.mu.Unlock()
	d.setState(statePairing)
	return ctx
}

func (d *daemon) pairFailed(err error) {
	d.log.Err(err).Msg("Pairing failed")
	msg := err.Error()
	switch {
	case errors.Is(err, libgm.ErrNoDevicesFound):
		msg = "No phone found for this Google account. In Google Messages on your phone, open Device pairing and make sure you're signed in with the same account."
	case errors.Is(err, libgm.ErrIncorrectEmoji):
		msg = "The wrong emoji was tapped. Try again."
	case errors.Is(err, libgm.ErrPairingCancelled):
		msg = "Pairing was cancelled on the phone."
	case errors.Is(err, libgm.ErrPairingTimeout), errors.Is(err, libgm.ErrPairingInitTimeout):
		msg = "The phone didn't respond in time. Make sure it's online, then try again."
	case errors.Is(err, context.Canceled):
		msg = "Pairing cancelled."
	}
	d.o.event("pair_error", map[string]string{"message": msg})
	d.setState(stateUnpaired)
}

func (d *daemon) paired(ad *libgm.AuthData) {
	d.saveSession(ad)
	d.o.event("paired", map[string]string{"account": ad.Mobile.GetSourceID()})
	go d.connect(context.Background(), ad)
}

func (d *daemon) pairGoogle(cookies map[string]string) error {
	if len(cookies) == 0 {
		return errors.New("no Google cookies")
	}
	ctx := d.beginPairing()
	ad := libgm.NewAuthData()
	ad.Cookies = cookies
	c := d.newClient(ad)
	c.SetEventHandler(func(evt any) { d.log.Debug().Type("type", evt).Msg("Pre-pairing event") })
	if err := c.FetchConfig(ctx); err != nil {
		d.pairFailed(fmt.Errorf("fetch config: %w", err))
		return err
	}
	emoji, sess, err := c.StartGaiaPairing(ctx, ctx)
	if err != nil {
		c.Disconnect()
		d.pairFailed(err)
		return err
	}
	d.o.event("pair_emoji", map[string]string{"emoji": emoji, "account": c.Config.GetDeviceInfo().GetEmail()})
	go func() {
		_, err := c.FinishGaiaPairing(ctx, sess)
		c.Disconnect()
		if err != nil {
			d.pairFailed(err)
			return
		}
		d.paired(c.AuthData)
	}()
	return nil
}

// ---- commands ----

func (d *daemon) handle(ctx context.Context, cmd string, raw json.RawMessage) (any, error) {
	var args struct {
		Cookies        map[string]string `json:"cookies"`
		ConversationID string            `json:"conversation_id"`
		MessageID      string            `json:"message_id"`
		Text           string            `json:"text"`
		TmpID          string            `json:"tmp_id"`
		ReplyTo        string            `json:"reply_to"`
		Count          int64             `json:"count"`
		Folder         string            `json:"folder"`
		BeforeID       string            `json:"before_id"`
		BeforeTS       int64             `json:"before_ts"`
		MediaID        string            `json:"media_id"`
		Key            string            `json:"key"`
		Mime           string            `json:"mime"`
		Emoji          string            `json:"emoji"`
		Remove         bool              `json:"remove"`
		Path           string            `json:"path"`
		IDs            []string          `json:"ids"`
		Number         string            `json:"number"`
		SIMNumber      *int32            `json:"sim_number"`
		Action         string            `json:"action"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
	}
	switch cmd {
	case "state":
		d.mu.Lock()
		defer d.mu.Unlock()
		return map[string]string{"state": d.state, "account": d.account}, nil
	case "sims":
		d.mu.Lock()
		defer d.mu.Unlock()
		return simChoices(d.settings), nil
	case "pair_google":
		return nil, d.pairGoogle(args.Cookies)
	case "check_cookies":
		// Validates Google cookies without starting a pairing.
		c := d.newClient(&libgm.AuthData{Cookies: args.Cookies})
		if err := c.FetchConfig(ctx); err != nil {
			return nil, err
		}
		return map[string]string{"account": c.Config.GetDeviceInfo().GetEmail()}, nil
	case "contacts_cached":
		return d.cachedContacts(), nil
	case "cancel_pair":
		d.mu.Lock()
		if d.pairCancel != nil {
			d.pairCancel()
		}
		d.mu.Unlock()
		return nil, nil
	case "logout":
		if c, err := d.connected(); err == nil {
			if err := c.Unpair(ctx); err != nil {
				d.log.Err(err).Msg("Unpair")
			}
		}
		d.loggedOut("Signed out.")
		return nil, nil
	}

	c, err := d.connected()
	if err != nil {
		return nil, err
	}
	switch cmd {
	case "conversations":
		folder := gmproto.ListConversationsRequest_INBOX
		if args.Folder == "archive" {
			folder = gmproto.ListConversationsRequest_ARCHIVE
		}
		if args.Count == 0 {
			args.Count = 75
		}
		resp, err := c.ListConversations(ctx, &gmproto.ListConversationsRequest{Count: args.Count, Folder: folder})
		if err != nil {
			return nil, err
		}
		d.cacheConversations(resp.GetConversations())
		convs := []Conversation{}
		for _, conv := range resp.GetConversations() {
			convs = append(convs, convertConversation(conv))
		}
		return convs, nil

	case "contacts":
		return d.contacts(ctx, c)

	case "messages":
		if args.Count == 0 {
			args.Count = 40
		}
		// Page backwards from the oldest message the app has, like the
		// bridge: the server's own cursor is sometimes missing.
		var cursor *gmproto.Cursor
		if args.BeforeTS > 0 {
			cursor = &gmproto.Cursor{LastItemID: args.BeforeID, LastItemTimestamp: args.BeforeTS}
		}
		resp, err := c.FetchMessages(ctx, args.ConversationID, args.Count, cursor)
		if err != nil {
			return nil, err
		}
		parts := d.participants(ctx, args.ConversationID)
		msgs := []Message{}
		for _, m := range resp.GetMessages() {
			if cursor != nil && (m.GetMessageID() == cursor.LastItemID || m.GetTimestamp()/1000 >= cursor.LastItemTimestamp) {
				continue
			}
			msgs = append(msgs, convertMessage(m, parts))
		}
		raw := len(resp.GetMessages())
		more := len(msgs) > 0 && (resp.GetCursor() != nil || int64(raw) >= args.Count)
		return map[string]any{"messages": msgs, "more": more}, nil

	case "send":
		conv, err := d.conversation(ctx, args.ConversationID)
		if err != nil {
			return nil, err
		}
		tmp := args.TmpID
		if tmp == "" {
			tmp = "tmp_" + uuid.NewString()
		}
		var infos []*gmproto.MessageInfo
		if args.Path != "" {
			media, err := d.upload(c, args.Path)
			if err != nil {
				return nil, fmt.Errorf("upload: %w", err)
			}
			infos = append(infos, &gmproto.MessageInfo{Data: &gmproto.MessageInfo_MediaContent{MediaContent: media}})
		}
		if args.Text != "" {
			infos = append(infos, &gmproto.MessageInfo{Data: &gmproto.MessageInfo_MessageContent{
				MessageContent: &gmproto.MessageContent{Content: args.Text},
			}})
		}
		if len(infos) == 0 {
			return nil, errors.New("nothing to send")
		}
		req := &gmproto.SendMessageRequest{
			ConversationID: args.ConversationID,
			MessagePayload: &gmproto.MessagePayload{
				TmpID:          tmp,
				ConversationID: args.ConversationID,
				ParticipantID:  conv.GetDefaultOutgoingID(),
				TmpID2:         tmp,
				MessageInfo:    infos,
			},
			SIMPayload: conv.GetSimCard().GetSIMData().GetSIMPayload(),
			TmpID:      tmp,
		}
		if args.SIMNumber != nil {
			d.mu.Lock()
			settings := d.settings
			d.mu.Unlock()
			var chosen *gmproto.SIMCard
			for _, card := range settings.GetSIMCards() {
				if card.GetSIMData().GetSIMPayload() != nil && card.GetSIMData().GetSIMPayload().GetSIMNumber() == *args.SIMNumber && card.GetSIMParticipant().GetID() != "" {
					chosen = card
					break
				}
			}
			if chosen == nil {
				return nil, errors.New("selected SIM is unavailable on the phone")
			}
			req.SIMPayload = chosen.GetSIMData().GetSIMPayload()
			req.MessagePayload.ParticipantID = chosen.GetSIMParticipant().GetID()
		}
		if args.ReplyTo != "" {
			req.Reply = &gmproto.ReplyPayload{MessageID: args.ReplyTo}
		}
		resp, err := c.SendMessage(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp.GetStatus() != gmproto.SendMessageResponse_SUCCESS {
			return nil, fmt.Errorf("phone refused the message (%s)", resp.GetStatus())
		}
		return map[string]string{"tmp_id": tmp}, nil

	case "new_conversation":
		// Open (or create) the one-to-one conversation with a number, as
		// the bridge does for a new chat.
		if args.Number == "" {
			return nil, errors.New("no phone number")
		}
		resp, err := c.GetOrCreateConversation(ctx, &gmproto.GetOrCreateConversationRequest{
			Numbers: []*gmproto.ContactNumber{{MysteriousInt: 2, Number: args.Number, Number2: args.Number}},
		})
		if err != nil {
			return nil, err
		}
		conv := resp.GetConversation()
		if conv == nil {
			return nil, fmt.Errorf("the phone couldn't start it (%s)", resp.GetStatus())
		}
		d.cacheConversations([]*gmproto.Conversation{conv})
		return convertConversation(conv), nil

	case "mark_read":
		return nil, c.MarkRead(ctx, args.ConversationID, args.MessageID)

	case "typing":
		conv, err := d.conversation(ctx, args.ConversationID)
		if err != nil {
			return nil, err
		}
		return nil, c.SetTyping(ctx, args.ConversationID, conv.GetSimCard().GetSIMData().GetSIMPayload())

	case "react":
		conv, err := d.conversation(ctx, args.ConversationID)
		if err != nil {
			return nil, err
		}
		action := gmproto.SendReactionRequest_ADD
		if args.Remove {
			action = gmproto.SendReactionRequest_REMOVE
		}
		_, err = c.SendReaction(ctx, &gmproto.SendReactionRequest{
			MessageID:    args.MessageID,
			ReactionData: gmproto.MakeReactionData(args.Emoji),
			Action:       action,
			SIMPayload:   conv.GetSimCard().GetSIMData().GetSIMPayload(),
		})
		return nil, err

	case "delete_message":
		resp, err := c.DeleteMessage(ctx, args.MessageID)
		if err != nil {
			return nil, err
		}
		if !resp.GetSuccess() {
			return nil, errors.New("the phone didn't delete it")
		}
		return nil, nil

	case "update_conversation":
		return nil, d.updateConversation(ctx, c, args.ConversationID, args.Action)

	case "download":
		return d.download(c, args.MediaID, args.Key, args.Mime)

	case "avatars":
		return d.avatars(ctx, c, args.IDs)
	}
	return nil, fmt.Errorf("unknown command %q", cmd)
}

func simChoices(settings *gmproto.Settings) []map[string]any {
	choices := make([]map[string]any, 0)
	for _, card := range settings.GetSIMCards() {
		data := card.GetSIMData()
		if data.GetSIMPayload() == nil || card.GetSIMParticipant().GetID() == "" {
			continue
		}
		number := data.GetSIMPayload().GetSIMNumber()
		label := data.GetCarrierName()
		if label == "" {
			label = fmt.Sprintf("SIM %d", number)
		}
		choices = append(choices, map[string]any{
			"number": number,
			"label":  label,
			"phone":  data.GetFormattedPhoneNumber(),
		})
	}
	return choices
}

func (d *daemon) download(c *libgm.Client, id, key64, mimeType string) (any, error) {
	if id == "" {
		return nil, errors.New("no media id")
	}
	ext := extensionFor(mimeType)
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '.' {
			return '_'
		}
		return r
	}, id)
	path := filepath.Join(d.dir, "media", safe+ext)
	if _, err := os.Stat(path); err == nil {
		return map[string]string{"path": path}, nil
	}
	key, err := base64.StdEncoding.DecodeString(key64)
	if err != nil {
		return nil, err
	}
	r, err := c.DownloadMedia(id, key)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	tmp := path + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmp)
		return nil, err
	}
	f.Close()
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}
	return map[string]string{"path": path}, nil
}

// avatars fetches contact photos for participant IDs and returns a path for
// each ID that has one ("" when the contact has no photo).
func (d *daemon) avatars(ctx context.Context, c *libgm.Client, ids []string) (any, error) {
	// Our own participant IDs, to recognise our photo if the phone hands it
	// out for someone else.
	selfIDs := map[string]bool{}
	d.mu.Lock()
	for _, conv := range d.convs {
		if id := conv.GetDefaultOutgoingID(); id != "" {
			selfIDs[id] = true
		}
	}
	d.mu.Unlock()
	request := append([]string(nil), ids...)
	for id := range selfIDs {
		request = append(request, id)
	}

	paths := map[string]string{}
	hashes := map[string][32]byte{}
	const batch = 20
	for start := 0; start < len(request); start += batch {
		chunk := request[start:min(start+batch, len(request))]
		wanted := map[string]bool{}
		for _, id := range chunk {
			wanted[id] = true
		}
		resp, err := c.GetParticipantThumbnail(ctx, chunk...)
		if err != nil {
			return nil, err
		}
		for _, t := range resp.GetThumbnail() {
			data := t.GetData().GetImageBuffer()
			if len(data) == 0 || !wanted[t.GetIdentifier()] {
				continue
			}
			path, sum, err := d.saveAvatar(data)
			if err != nil {
				return nil, err
			}
			paths[t.GetIdentifier()] = path
			hashes[t.GetIdentifier()] = sum
		}
	}

	ours := map[[32]byte]bool{}
	for id := range selfIDs {
		if h, ok := hashes[id]; ok {
			ours[h] = true
		}
	}
	out := map[string]string{}
	for _, id := range ids {
		if h, ok := hashes[id]; ok && ours[h] && !selfIDs[id] {
			d.log.Debug().Msg("Dropping a contact photo that is our own")
			out[id] = ""
			continue
		}
		out[id] = paths[id]
	}
	return out, nil
}

// saveAvatar stores a photo content-addressed, so a changed photo gets a new
// path and the app's image cache never shows the old one.
func (d *daemon) saveAvatar(data []byte) (string, [32]byte, error) {
	ext := ".jpg"
	switch http.DetectContentType(data) {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	}
	sum := sha256.Sum256(data)
	path := filepath.Join(d.dir, "media", "avatar_"+hex.EncodeToString(sum[:8])+ext)
	if _, err := os.Stat(path); err != nil {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return "", sum, err
		}
	}
	return path, sum, nil
}

func (d *daemon) upload(c *libgm.Client, path string) (*gmproto.MediaContent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return c.UploadMedia(data, filepath.Base(path), mimeType)
}

// extensionFor picks the usual extension for a MIME type. The app's image
// decoder goes by extension, and Go's table lists rare ones (".pjpeg") too.
func extensionFor(mimeType string) string {
	base, _, _ := strings.Cut(strings.ToLower(mimeType), ";")
	switch strings.TrimSpace(base) {
	case "image/jpeg", "image/jpg", "image/pjpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/heic", "image/heif":
		return ".heic"
	case "image/bmp":
		return ".bmp"
	case "video/mp4":
		return ".mp4"
	case "video/3gpp":
		return ".3gp"
	case "audio/mpeg":
		return ".mp3"
	case "audio/amr":
		return ".amr"
	case "audio/ogg":
		return ".ogg"
	case "application/pdf":
		return ".pdf"
	case "text/x-vcard", "text/vcard":
		return ".vcf"
	}
	if exts, _ := mime.ExtensionsByType(base); len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

// updateConversation archives, mutes, blocks or deletes a conversation on
// the phone. The phone answers with a conversation update event.
func (d *daemon) updateConversation(ctx context.Context, c *libgm.Client, id, action string) error {
	if id == "" {
		return errors.New("no conversation")
	}
	status := func(st gmproto.ConversationStatus) *gmproto.UpdateConversationRequest {
		return &gmproto.UpdateConversationRequest{
			ConversationID: id,
			Data: &gmproto.UpdateConversationRequest_UpdateData{UpdateData: &gmproto.UpdateConversationData{
				ConversationID: id, Data: &gmproto.UpdateConversationData_Status{Status: st},
			}},
		}
	}
	mute := func(m gmproto.ConversationMuteStatus) *gmproto.UpdateConversationRequest {
		return &gmproto.UpdateConversationRequest{
			ConversationID: id,
			Data: &gmproto.UpdateConversationRequest_UpdateData{UpdateData: &gmproto.UpdateConversationData{
				ConversationID: id, Data: &gmproto.UpdateConversationData_Mute{Mute: m},
			}},
		}
	}
	var req *gmproto.UpdateConversationRequest
	switch action {
	case "archive":
		req = status(gmproto.ConversationStatus_ARCHIVED)
	case "unarchive":
		req = status(gmproto.ConversationStatus_ACTIVE)
	case "mute":
		req = mute(gmproto.ConversationMuteStatus_MUTE)
	case "unmute":
		req = mute(gmproto.ConversationMuteStatus_UNMUTE)
	case "block_spam":
		req = &gmproto.UpdateConversationRequest{
			Action:         gmproto.ConversationActionStatus_BLOCK_AND_REPORT,
			ConversationID: id,
			Action5:        &gmproto.ConversationAction5{Field2: true},
		}
	case "delete":
		return c.DeleteConversation(ctx, id, d.phoneOf(ctx, id))
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	resp, err := c.UpdateConversation(ctx, req)
	if err != nil {
		return err
	}
	if !resp.GetSuccess() {
		return errors.New("the phone refused it")
	}
	return nil
}

// phoneOf is the other number of a one-to-one chat, which delete wants.
func (d *daemon) phoneOf(ctx context.Context, id string) string {
	conv, err := d.conversation(ctx, id)
	if err != nil {
		return ""
	}
	for _, p := range conv.GetParticipants() {
		if p.GetIsVisible() && !p.GetIsMe() && p.GetID().GetNumber() != "" {
			return p.GetID().GetNumber()
		}
	}
	return ""
}

// ---- contacts ----

// The phone answers LIST_CONTACTS slowly or not at all at times (right after
// a contact change, or while the session is still coming up). The UI shows
// the last list from disk at once and refreshes it in the background, so
// these timeouts only bound how long the refresh spinner runs.
const (
	contactsTimeout    = 20 * time.Second
	topContactsTimeout = 5 * time.Second
)

func (d *daemon) contactsPath() string { return filepath.Join(d.dir, "contacts.json") }

func (d *daemon) contacts(ctx context.Context, c *libgm.Client) ([]map[string]any, error) {
	// Top contacts only rank the list: fetch them alongside, never wait long.
	topCh := make(chan map[string]bool, 1)
	go func() {
		tctx, cancel := context.WithTimeout(ctx, topContactsTimeout)
		defer cancel()
		ids := map[string]bool{}
		top, err := c.ListTopContacts(tctx)
		if err != nil {
			d.log.Debug().Err(err).Msg("Top contacts unavailable")
		}
		for _, contact := range top.GetContacts() {
			ids[contact.GetParticipantID()] = true
		}
		topCh <- ids
	}()

	lctx, cancel := context.WithTimeout(ctx, contactsTimeout)
	defer cancel()
	resp, err := c.ListContacts(lctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, errors.New("the phone didn't send its contacts in time")
	} else if err != nil {
		return nil, err
	}
	var topIDs map[string]bool
	select {
	case topIDs = <-topCh:
	case <-time.After(topContactsTimeout):
	}

	contacts := make([]map[string]any, 0, len(resp.GetContacts()))
	for _, contact := range resp.GetContacts() {
		number := contact.GetNumber().GetNumber()
		if number == "" {
			continue
		}
		name := contact.GetName()
		if name == "" {
			name = contact.GetNameAgain()
		}
		if name == "" {
			name = number
		}
		formatted := contact.GetNumber().GetFormattedNumber()
		if formatted == "" {
			formatted = number
		}
		contacts = append(contacts, map[string]any{
			"name": name, "number": number, "formatted_number": formatted,
			"top": topIDs[contact.GetParticipantID()], "avatar_id": contact.GetParticipantID(),
		})
	}
	d.saveContacts(contacts)
	return contacts, nil
}

func (d *daemon) saveContacts(contacts []map[string]any) {
	data, err := json.Marshal(contacts)
	if err != nil {
		return
	}
	tmp := d.contactsPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err == nil {
		err = os.Rename(tmp, d.contactsPath())
	}
	if err != nil {
		d.log.Err(err).Msg("Save contacts")
	}
}

func (d *daemon) cachedContacts() []map[string]any {
	data, err := os.ReadFile(d.contactsPath())
	if err != nil {
		return nil
	}
	var contacts []map[string]any
	if json.Unmarshal(data, &contacts) != nil {
		return nil
	}
	return contacts
}
