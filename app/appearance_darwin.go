//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

// 0 follows the system, 1 is light, 2 is dark.
static void setWindowAppearance(void* nativeWindow, int mode) {
	if (!nativeWindow) return;
	void (^change)(void) = ^{
		NSWindow* window = (NSWindow*)nativeWindow;
		window.appearance = mode == 1 ? [NSAppearance appearanceNamed:NSAppearanceNameAqua]
			: mode == 2 ? [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua]
			: nil;
	};
	if ([NSThread isMainThread]) change();
	else dispatch_sync(dispatch_get_main_queue(), change);
}

// Transparency: the system's behind-window blur, in a view under the page.
// Being part of the window, it follows the window's rounded corners. The
// window and the web view are cleared so the page's translucent panes show
// it; material picks the system look (0 hides the blur).
static void setWindowBlur(void* nativeWindow, int material) {
	if (!nativeWindow) return;
	void (^change)(void) = ^{
		NSWindow* window = (NSWindow*)nativeWindow;
		// Clear all the way down: the window, and the web view's own white.
		window.opaque = NO;
		window.backgroundColor = NSColor.clearColor;
		NSVisualEffectView* effect = nil;
		NSMutableArray<NSView*>* views = [NSMutableArray arrayWithObject:window.contentView];
		while (views.count) {
			NSView* view = views.lastObject;
			[views removeLastObject];
			if ([view isKindOfClass:[WKWebView class]]) {
				[view setValue:@NO forKey:@"drawsBackground"];
				((WKWebView*)view).underPageBackgroundColor = NSColor.clearColor;
			}
			if ([view isKindOfClass:[NSVisualEffectView class]] && !effect) effect = (NSVisualEffectView*)view;
			[views addObjectsFromArray:view.subviews];
		}
		if (!effect && material > 0) {
			NSView* content = window.contentView;
			effect = [[NSVisualEffectView alloc] initWithFrame:content.bounds];
			effect.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
			[content addSubview:effect positioned:NSWindowBelow relativeTo:nil];
		}
		if (effect) {
			effect.blendingMode = NSVisualEffectBlendingModeBehindWindow;
			effect.state = NSVisualEffectStateActive;
			if (material > 0) effect.material = (NSVisualEffectMaterial)material;
			effect.hidden = material <= 0;
		}
	};
	if ([NSThread isMainThread]) change();
	else dispatch_sync(dispatch_get_main_queue(), change);
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

// setWindowBlur shows the system blur behind the window, or hides it.
func setWindowBlur(window *application.WebviewWindow, on bool) {
	material := 0
	if on {
		material = blurMaterial
	}
	C.setWindowBlur(window.NativeWindow(), C.int(material))
}

// blurMaterial is NSVisualEffectMaterialFullScreenUI: of the behind-window
// materials it lets the most of the desktop through (the window and
// sidebar ones draw nearly opaque white in light mode).
const blurMaterial = 15

// setAppearance tints the window chrome (traffic lights, scroll bars)
// light or dark to match the theme: "light", "dark", or "" for the system.
func setAppearance(window *application.WebviewWindow, mode string) {
	m := 0
	switch mode {
	case "light":
		m = 1
	case "dark":
		m = 2
	}
	C.setWindowAppearance(window.NativeWindow(), C.int(m))
}
