package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// LinkPreview is what a link chip and its hover card show. Image and Icon
// are files in the media folder, served at /media/<file> like photos.
type LinkPreview struct {
	URL         string `json:"url"` // where the link ended up, after redirects
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Site        string `json:"site"`
	Image       string `json:"image,omitempty"`
	Icon        string `json:"icon,omitempty"`
	FetchedAt   int64  `json:"fetched_at"` // unix seconds
}

const (
	linkTimeout   = 6 * time.Second
	linkHTMLLimit = 512 << 10 // the <head> is near the top; no need for the whole page
	linkImageMax  = 3 << 20
	linkIconMax   = 200 << 10
	linkKeep      = 7 * 24 * time.Hour // a page's title rarely changes in a week
	linkRetry     = 10 * time.Minute   // a failed page isn't asked again before this
	linkUserAgent = "Mozilla/5.0 (compatible; GoMessages link preview)"
)

// links fetches and caches previews. Pages are fetched by Go, never the web
// view: no cookies, no scripts, and nothing on the local network.
type links struct {
	dir    string // the media folder; previews are kept in dir/../links
	client *http.Client

	mu       sync.Mutex
	inflight map[string]chan struct{}
	failed   map[string]time.Time
}

func newLinks(dataDir string) *links {
	l := &links{
		dir:      filepath.Join(dataDir, "media"),
		inflight: map[string]chan struct{}{},
		failed:   map[string]time.Time{},
	}
	dialer := &net.Dialer{Timeout: linkTimeout, Control: publicOnly}
	l.client = &http.Client{
		Timeout: linkTimeout,
		Transport: &http.Transport{
			// No system proxy: the dial check below must see the real address.
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   linkTimeout,
			ResponseHeaderTimeout: linkTimeout,
			MaxIdleConns:          8,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("redirect to a non-web address")
			}
			return nil
		},
	}
	_ = os.MkdirAll(filepath.Join(dataDir, "links"), 0o700)
	return l
}

// publicOnly refuses to connect anywhere on this computer or its network.
// It runs on the address actually dialed, after DNS, so a public name that
// resolves to 192.168.x.x (or changes between lookups) is refused too. A
// link in a text from a stranger must not be able to reach the router.
func publicOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !isPublicIP(ip) {
		return fmt.Errorf("link preview: %s is not a public address", host)
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	// Carrier-grade NAT (100.64/10), which Tailscale and some ISPs use.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64 {
		return false
	}
	return true
}

func linkKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:12])
}

func (l *links) cachePath(raw string) string {
	return filepath.Join(filepath.Dir(l.dir), "links", linkKey(raw)+".json")
}

// preview returns the page's preview, from the cache when it's recent.
// Concurrent asks for the same link share one fetch.
func (l *links) preview(ctx context.Context, raw string) (*LinkPreview, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, errors.New("not a web link")
	}
	if p := l.cached(raw); p != nil {
		return p, nil
	}
	for {
		l.mu.Lock()
		if t, ok := l.failed[raw]; ok && time.Since(t) < linkRetry {
			l.mu.Unlock()
			return nil, errors.New("no preview")
		}
		wait, busy := l.inflight[raw]
		if !busy {
			done := make(chan struct{})
			l.inflight[raw] = done
			l.mu.Unlock()
			p, err := l.fetch(ctx, u)
			l.mu.Lock()
			delete(l.inflight, raw)
			if err != nil {
				l.failed[raw] = time.Now()
			}
			l.mu.Unlock()
			close(done)
			if err == nil {
				l.store(raw, p)
			}
			return p, err
		}
		l.mu.Unlock()
		select {
		case <-wait:
			if p := l.cached(raw); p != nil {
				return p, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (l *links) cached(raw string) *LinkPreview {
	data, err := os.ReadFile(l.cachePath(raw))
	if err != nil {
		return nil
	}
	var p LinkPreview
	if json.Unmarshal(data, &p) != nil || time.Since(time.Unix(p.FetchedAt, 0)) > linkKeep {
		return nil
	}
	return &p
}

func (l *links) store(raw string, p *LinkPreview) {
	data, err := json.Marshal(p)
	if err != nil {
		return
	}
	tmp := l.cachePath(raw) + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, l.cachePath(raw))
	}
}

func (l *links) get(ctx context.Context, u *url.URL, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", linkUserAgent)
	req.Header.Set("Accept", accept)
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", u.Host, resp.Status)
	}
	return resp, nil
}

func (l *links) fetch(ctx context.Context, u *url.URL) (*LinkPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*linkTimeout)
	defer cancel()
	resp, err := l.get(ctx, u, "text/html,application/xhtml+xml;q=0.9,image/*;q=0.8,*/*;q=0.1")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	final := resp.Request.URL
	p := &LinkPreview{URL: final.String(), Site: siteName(final), FetchedAt: time.Now().Unix()}
	kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))

	// A link straight to a picture: the picture is the preview.
	if strings.HasPrefix(kind, "image/") {
		p.Title = path.Base(final.Path)
		p.Image = l.save(resp, linkKey(p.URL), linkImageMax, imageExt)
		if p.Image == "" {
			return nil, errors.New("image too large or unknown")
		}
		return p, nil
	}
	if kind != "text/html" && kind != "application/xhtml+xml" {
		return nil, fmt.Errorf("not a web page (%s)", kind)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, linkHTMLLimit))
	if err != nil && len(body) == 0 {
		return nil, err
	}
	meta := parseHead(string(body))
	p.Title = clip(meta.title, 200)
	p.Description = clip(meta.description, 300)
	if meta.site != "" {
		p.Site = clip(meta.site, 60)
	}
	if p.Title == "" && p.Description == "" {
		return nil, errors.New("page has no title")
	}

	// The picture and the icon are extras: a page still previews without them.
	if img := resolve(final, meta.image); img != nil {
		if r, err := l.get(ctx, img, "image/*"); err == nil {
			p.Image = l.save(r, linkKey(p.URL), linkImageMax, imageExt)
			r.Body.Close()
		}
	}
	icon := resolve(final, meta.icon)
	if icon == nil {
		icon = &url.URL{Scheme: final.Scheme, Host: final.Host, Path: "/favicon.ico"}
	}
	if r, err := l.get(ctx, icon, "image/*"); err == nil {
		p.Icon = l.save(r, linkKey(p.URL)+"-icon", linkIconMax, iconExt)
		r.Body.Close()
	}
	return p, nil
}

var (
	imageExt = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp"}
	iconExt  = map[string]string{
		"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp",
		"image/x-icon": ".ico", "image/vnd.microsoft.icon": ".ico", "image/svg+xml": ".svg",
	}
)

// save writes a picture to the media folder and returns its path, or ""
// when it's too big or not a format the web view shows in an <img>.
func (l *links) save(resp *http.Response, name string, limit int64, exts map[string]string) string {
	kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	ext, ok := exts[kind]
	if !ok || resp.ContentLength > limit {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(data)) > limit || len(data) == 0 {
		return ""
	}
	file := filepath.Join(l.dir, "lp-"+name+ext)
	if os.WriteFile(file, data, 0o600) != nil {
		return ""
	}
	return file
}

func siteName(u *url.URL) string {
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

func resolve(base *url.URL, ref string) *url.URL {
	if ref == "" {
		return nil
	}
	u, err := base.Parse(ref)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	return u
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(strings.ToValidUTF8(s, "")), " ")
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n-1])) + "…"
	}
	return s
}

type pageMeta struct {
	title, description, site, image, icon string
}

var (
	tagRE   = regexp.MustCompile(`(?is)<(meta|link)\b([^>]*)>`)
	titleRE = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)
	attrRE  = regexp.MustCompile(`(?s)([a-zA-Z_:.-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	bodyRE  = regexp.MustCompile(`(?i)<body\b`)
)

// parseHead reads Open Graph, Twitter card and plain HTML tags from the top
// of a page. A regexp over <meta> and <link> is enough here: we only want a
// handful of attributes, and a page that confuses it just gets no preview.
func parseHead(page string) pageMeta {
	// Tags in the body are the page's content, not its description.
	if loc := bodyRE.FindStringIndex(page); loc != nil {
		page = page[:loc[0]]
	}
	props := map[string]string{}
	var icons []map[string]string
	for _, m := range tagRE.FindAllStringSubmatch(page, -1) {
		attrs := map[string]string{}
		for _, a := range attrRE.FindAllStringSubmatch(m[2], -1) {
			attrs[strings.ToLower(a[1])] = html.UnescapeString(a[2] + a[3] + a[4])
		}
		if strings.EqualFold(m[1], "link") {
			if strings.Contains(strings.ToLower(attrs["rel"]), "icon") && attrs["href"] != "" {
				icons = append(icons, attrs)
			}
			continue
		}
		key := strings.ToLower(attrs["property"])
		if key == "" {
			key = strings.ToLower(attrs["name"])
		}
		if key != "" && attrs["content"] != "" {
			if _, seen := props[key]; !seen {
				props[key] = strings.TrimSpace(attrs["content"])
			}
		}
	}
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := props[k]; v != "" {
				return v
			}
		}
		return ""
	}
	var meta pageMeta
	meta.title = first("og:title", "twitter:title")
	if meta.title == "" {
		if m := titleRE.FindStringSubmatch(page); m != nil {
			meta.title = html.UnescapeString(m[1])
		}
	}
	meta.description = first("og:description", "twitter:description", "description")
	meta.site = first("og:site_name", "application-name")
	meta.image = first("og:image:secure_url", "og:image", "og:image:url", "twitter:image", "twitter:image:src")
	// A plain "icon" first; Apple's touch icon is bigger but often missing.
	for _, pass := range []func(rel string) bool{
		func(rel string) bool { return !strings.Contains(rel, "apple") && !strings.Contains(rel, "mask") },
		func(rel string) bool { return true },
	} {
		for _, ic := range icons {
			if pass(strings.ToLower(ic["rel"])) {
				meta.icon = ic["href"]
				break
			}
		}
		if meta.icon != "" {
			break
		}
	}
	return meta
}
