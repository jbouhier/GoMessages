//go:build linux

package main

/*
#cgo !gtk3 pkg-config: webkitgtk-6.0
#cgo gtk3 pkg-config: webkit2gtk-4.1
#cgo gtk3 CFLAGS: -DGM_GTK3
#ifdef GM_GTK3
#include <webkit2/webkit2.h>
#else
#include <webkit/webkit.h>
#endif
#include <stdlib.h>
#include <string.h>

// One GetCookies call: filled in on the GTK main loop, read from Go once done.
typedef struct {
	gint done;
	char *lines; // "name\tvalue\tdomain\n" per cookie
} gm_cookies;

static char *gm_webview_uri(void *webview) {
	const char *uri = webkit_web_view_get_uri(WEBKIT_WEB_VIEW(webview));
	return uri ? strdup(uri) : NULL;
}

static void gm_cookies_ready(GObject *source, GAsyncResult *res, gpointer data) {
	gm_cookies *r = data;
	GString *out = g_string_new(NULL);
	GList *cookies = webkit_cookie_manager_get_cookies_finish(WEBKIT_COOKIE_MANAGER(source), res, NULL);
	for (GList *l = cookies; l; l = l->next) {
		SoupCookie *c = l->data;
		g_string_append_printf(out, "%s\t%s\t%s\n", soup_cookie_get_name(c), soup_cookie_get_value(c), soup_cookie_get_domain(c));
	}
	g_list_free_full(cookies, (GDestroyNotify)soup_cookie_free);
	r->lines = g_string_free(out, FALSE);
	g_atomic_int_set(&r->done, 1);
}

// gm_get_cookies starts reading; call it on the GTK main thread.
static gm_cookies *gm_get_cookies(void *webview) {
	WebKitWebView *wv = WEBKIT_WEB_VIEW(webview);
#ifdef GM_GTK3
	WebKitCookieManager *manager = webkit_web_context_get_cookie_manager(webkit_web_view_get_context(wv));
#else
	WebKitCookieManager *manager = webkit_network_session_get_cookie_manager(webkit_web_view_get_network_session(wv));
#endif
	gm_cookies *r = g_new0(gm_cookies, 1);
	webkit_cookie_manager_get_cookies(manager, "https://messages.google.com/web/config", NULL, gm_cookies_ready, r);
	return r;
}

static int gm_cookies_done(gm_cookies *r) { return g_atomic_int_get(&r->done); }

static void gm_cookies_free(gm_cookies *r) {
	g_free(r->lines);
	g_free(r);
}
*/
import "C"

import (
	"errors"
	"strings"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// webkitWebView is the sign-in window's WebKitWebView.
func webkitWebView(w *application.WebviewWindow) unsafe.Pointer {
	return webviewHandle(w, "webview")
}

func readWebviewURL(w *application.WebviewWindow) string {
	wv := webkitWebView(w)
	if wv == nil {
		return ""
	}
	cURL := application.InvokeSyncWithResult(func() *C.char { return C.gm_webview_uri(wv) })
	if cURL == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cURL))
	return C.GoString(cURL)
}

func readWebviewGoogleCookies(w *application.WebviewWindow) (map[string]string, error) {
	wv := webkitWebView(w)
	if wv == nil {
		return nil, errors.New("Google sign-in window is closed. Open it again")
	}
	r := application.InvokeSyncWithResult(func() *C.gm_cookies { return C.gm_get_cookies(wv) })
	for deadline := time.Now().Add(10 * time.Second); C.gm_cookies_done(r) == 0; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			// Still pending: GTK writes into r later, so it isn't freed.
			return nil, errors.New("could not read cookies from the Google sign-in window")
		}
	}
	lines := C.GoString(r.lines)
	C.gm_cookies_free(r)
	cookies := map[string]string{}
	for _, line := range strings.Split(lines, "\n") {
		if f := strings.Split(line, "\t"); len(f) == 3 && keepGoogleCookie(f[0], f[1], f[2]) {
			cookies[f[0]] = f[1]
		}
	}
	return checkGoogleCookies(cookies)
}
