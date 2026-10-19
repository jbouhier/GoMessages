//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <math.h>
#include <stdbool.h>

static void setPageZoom(void* nativeWindow, double zoom) {
	if (!nativeWindow) return;
	void (^change)(void) = ^{
		NSWindow* window = (NSWindow*)nativeWindow;
		if (![window respondsToSelector:@selector(webView)]) return;
		WKWebView* webView = [window valueForKey:@"webView"];
		if (!webView) return;
		// Page zoom reflows the page; Wails' default magnification only scales it.
		[webView setMagnification:1.0];
		webView.pageZoom = fmax(0.5, fmin(3.0, zoom));
	};
	if ([NSThread isMainThread]) change();
	else dispatch_sync(dispatch_get_main_queue(), change);
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

// setContentZoom sets the page zoom (the app's text size).
func setContentZoom(window *application.WebviewWindow, zoom float64) {
	C.setPageZoom(window.NativeWindow(), C.double(zoom))
}
