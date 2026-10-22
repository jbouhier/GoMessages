package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestParseHead(t *testing.T) {
	page := `<!doctype html><html><head>
	<title>Plain &amp; title</title>
	<meta property="og:title" content="OG &quot;title&quot;">
	<meta name="description" content="Plain description">
	<meta property='og:description' content='OG description'>
	<meta property="og:site_name" content="Example">
	<meta content="/img/card.png" property="og:image">
	<link rel="apple-touch-icon" href="/apple.png">
	<link rel="icon" type="image/png" href="/favicon-32.png">
	</head><body><meta property="og:title" content="from the body"></body></html>`
	m := parseHead(page)
	if m.title != `OG "title"` {
		t.Errorf("title = %q", m.title)
	}
	if m.description != "OG description" || m.site != "Example" || m.image != "/img/card.png" {
		t.Errorf("meta = %+v", m)
	}
	if m.icon != "/favicon-32.png" {
		t.Errorf("icon = %q, want the plain icon before Apple's", m.icon)
	}

	// No Open Graph: the <title> and the plain description.
	m = parseHead(`<html><head><TITLE>
	  Just a page </TITLE><meta name=description content=Short></head>`)
	if clip(m.title, 200) != "Just a page" || m.description != "Short" {
		t.Errorf("fallback = %+v", m)
	}
}

func TestClip(t *testing.T) {
	if got := clip("a  b\n\tc", 10); got != "a b c" {
		t.Errorf("clip spaces = %q", got)
	}
	if got := clip(strings.Repeat("é", 20), 10); got != strings.Repeat("é", 9)+"…" {
		t.Errorf("clip runes = %q", got)
	}
}

func TestIsPublicIP(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.0.0.8", "192.168.1.1", "172.16.4.4", "169.254.169.254", "100.100.1.1", "::1", "fe80::1", "fd00::1", "0.0.0.0"} {
		if isPublicIP(net.ParseIP(s)) {
			t.Errorf("%s counted as public", s)
		}
	}
	for _, s := range []string{"1.1.1.1", "93.184.216.34", "2606:4700::1111"} {
		if !isPublicIP(net.ParseIP(s)) {
			t.Errorf("%s counted as private", s)
		}
	}
}

func TestPreviewRefusesLocalNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the local server was reached")
	}))
	defer srv.Close()
	l := newLinks(t.TempDir())
	if _, err := l.preview(context.Background(), srv.URL); err == nil {
		t.Fatal("previewed a local address")
	}
	if _, err := l.preview(context.Background(), "file:///etc/passwd"); err == nil {
		t.Fatal("previewed a file")
	}
}

func TestPreviewFetchesAndCaches(t *testing.T) {
	hits := 0
	png := []byte("\x89PNG\r\n\x1a\nfake")
	mux := http.NewServeMux()
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("Cookie") != "" {
			t.Error("sent a cookie")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<head><meta property="og:title" content="Hello"><meta property="og:image" content="/card.png"></head>`))
	})
	mux.HandleFunc("/card.png", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := t.TempDir()
	l := newLinks(dir)
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// The test server is on 127.0.0.1, which the real dialer refuses.
	l.client.Transport = http.DefaultTransport

	p, err := l.preview(context.Background(), srv.URL+"/page")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Hello" || p.Site != "127.0.0.1" || p.Image == "" {
		t.Fatalf("preview = %+v", p)
	}
	if data, _ := os.ReadFile(p.Image); string(data) != string(png) {
		t.Errorf("image not saved")
	}
	if _, err := l.preview(context.Background(), srv.URL+"/page"); err != nil || hits != 1 {
		t.Errorf("second ask fetched again (hits %d, err %v)", hits, err)
	}
}
