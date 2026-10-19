//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <stdlib.h>
#include <string.h>

static char* webviewWindowURL(void* nativeWindow) {
	if (!nativeWindow) return NULL;
	__block char* result = NULL;
	void (^readURL)(void) = ^{
		NSWindow* window = (NSWindow*)nativeWindow;
		if (![window respondsToSelector:@selector(webView)]) return;
		WKWebView* webView = [window valueForKey:@"webView"];
		NSString* url = webView.URL.absoluteString;
		if (url) result = strdup(url.UTF8String);
	};
	if ([NSThread isMainThread]) readURL();
	else dispatch_sync(dispatch_get_main_queue(), readURL);
	return result;
}

static char* googleCookiesFromWindow(void* nativeWindow) {
	if (!nativeWindow) return NULL;
	__block char* result = NULL;
	dispatch_semaphore_t done = dispatch_semaphore_create(0);
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow* window = (NSWindow*)nativeWindow;
		if (![window respondsToSelector:@selector(webView)]) {
			dispatch_semaphore_signal(done);
			return;
		}
		WKWebView* webView = [window valueForKey:@"webView"];
		if (!webView) {
			dispatch_semaphore_signal(done);
			return;
		}
		WKHTTPCookieStore* store = webView.configuration.websiteDataStore.httpCookieStore;
		[store getAllCookies:^(NSArray<NSHTTPCookie*>* allCookies) {
			NSMutableDictionary* found = [NSMutableDictionary dictionary];
			NSDate* now = [NSDate date];
			NSSet* names = [NSSet setWithArray:@[@"SID", @"HSID", @"SSID", @"OSID", @"APISID", @"SAPISID", @"__Secure-1PSIDTS"]];
			for (NSHTTPCookie* cookie in allCookies) {
				NSString* host = cookie.domain;
				BOOL matches = [host isEqualToString:@".google.com"] || [host isEqualToString:@".messages.google.com"] || [host isEqualToString:@"messages.google.com"];
				if (matches && [names containsObject:cookie.name] && cookie.value.length > 0 && (!cookie.expiresDate || [cookie.expiresDate compare:now] == NSOrderedDescending)) {
					found[cookie.name] = cookie.value;
				}
			}
			NSData* data = [NSJSONSerialization dataWithJSONObject:found options:0 error:nil];
			if (data) {
				NSString* json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
				if (json) result = strdup(json.UTF8String);
				[json release];
			}
			dispatch_semaphore_signal(done);
		}];
	});
	dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
	return result;
}
*/
import "C"

import (
	"encoding/json"
	"errors"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func readWebviewURL(w *application.WebviewWindow) string {
	cURL := C.webviewWindowURL(w.NativeWindow())
	if cURL == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cURL))
	return C.GoString(cURL)
}

func readWebviewGoogleCookies(w *application.WebviewWindow) (map[string]string, error) {
	window := w.NativeWindow()
	if window == nil {
		return nil, errors.New("Google sign-in window is closed. Open it again")
	}
	// Keep the native cookie-store call off the macOS main thread.
	result := make(chan *C.char, 1)
	go func() { result <- C.googleCookiesFromWindow(window) }()
	cJSON := <-result
	if cJSON == nil {
		return nil, errors.New("could not read cookies from the Google sign-in window")
	}
	defer C.free(unsafe.Pointer(cJSON))
	var cookies map[string]string
	if err := json.Unmarshal([]byte(C.GoString(cJSON)), &cookies); err != nil {
		return nil, errors.New("could not decode Google sign-in cookies")
	}
	return checkGoogleCookies(cookies)
}
