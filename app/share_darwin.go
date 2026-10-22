//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <stdlib.h>

// Adds "Save as File…" and "Add to Contacts" to the system share menu.
@interface GMShareDelegate : NSObject <NSSharingServicePickerDelegate>
@property (strong) NSURL* file;
@end

@implementation GMShareDelegate
- (NSArray<NSSharingService*>*)sharingServicePicker:(NSSharingServicePicker*)picker
                            sharingServicesForItems:(NSArray*)items
                            proposedSharingServices:(NSArray<NSSharingService*>*)proposed {
	NSURL* file = self.file;
	NSImage* saveIcon = nil;
	NSImage* contactsIcon = nil;
	if (@available(macOS 11.0, *)) {
		saveIcon = [NSImage imageWithSystemSymbolName:@"square.and.arrow.down" accessibilityDescription:nil];
		contactsIcon = [NSImage imageWithSystemSymbolName:@"person.crop.circle.badge.plus" accessibilityDescription:nil];
	}
	NSSharingService* save = [[NSSharingService alloc] initWithTitle:@"Save as File…" image:saveIcon alternateImage:nil handler:^{
		NSSavePanel* panel = [NSSavePanel savePanel];
		panel.nameFieldStringValue = file.lastPathComponent;
		if ([panel runModal] != NSModalResponseOK || !panel.URL) return;
		NSFileManager* fm = [NSFileManager defaultManager];
		[fm removeItemAtURL:panel.URL error:nil];
		[fm copyItemAtURL:file toURL:panel.URL error:nil];
	}];
	NSSharingService* contacts = [[NSSharingService alloc] initWithTitle:@"Add to Contacts" image:contactsIcon alternateImage:nil handler:^{
		NSURL* app = [[NSWorkspace sharedWorkspace] URLForApplicationWithBundleIdentifier:@"com.apple.AddressBook"];
		if (app) {
			[[NSWorkspace sharedWorkspace] openURLs:@[file] withApplicationAtURL:app
			                         configuration:[NSWorkspaceOpenConfiguration configuration] completionHandler:nil];
		} else {
			[[NSWorkspace sharedWorkspace] openURL:file];
		}
	}];
	return [@[save, contacts] arrayByAddingObjectsFromArray:proposed];
}
@end

static GMShareDelegate* shareDelegate;

// Shows the share menu for a file under a rect given in the web view's points.
static void showShareMenu(void* nativeWindow, const char* path, double x, double y, double w, double h) {
	if (!nativeWindow) return;
	NSString* p = [NSString stringWithUTF8String:path];
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow* window = (NSWindow*)nativeWindow;
		NSView* view = window.contentView;
		if ([window respondsToSelector:@selector(webView)]) {
			WKWebView* webView = [window valueForKey:@"webView"];
			if (webView) view = webView;
		}
		NSURL* file = [NSURL fileURLWithPath:p];
		if (!shareDelegate) shareDelegate = [GMShareDelegate new];
		shareDelegate.file = file;
		NSSharingServicePicker* picker = [[NSSharingServicePicker alloc] initWithItems:@[file]];
		picker.delegate = shareDelegate;
		// The web view is flipped: its y runs down like the page's.
		NSRect r = NSMakeRect(x, view.isFlipped ? y : view.bounds.size.height - y - h, w, h);
		[picker showRelativeToRect:r ofView:view preferredEdge:view.isFlipped ? NSRectEdgeMaxY : NSRectEdgeMinY];
	});
}
*/
import "C"

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func shareSupported() bool { return true }

// showShare opens the system share menu for path under the given rect.
func showShare(window *application.WebviewWindow, path string, x, y, w, h float64) {
	cs := C.CString(path)
	defer C.free(unsafe.Pointer(cs))
	C.showShareMenu(window.NativeWindow(), cs, C.double(x), C.double(y), C.double(w), C.double(h))
}
