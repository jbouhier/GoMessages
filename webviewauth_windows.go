//go:build windows

package main

import (
	"errors"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

// WebView2 COM vtable slots, from WebView2.h. IUnknown takes 0–2.
const (
	slotQueryInterface   = 0
	slotRelease          = 2
	slotGetSource        = 4  // ICoreWebView2::get_Source
	slotGetCookieManager = 66 // ICoreWebView2_2::get_CookieManager
	slotGetCookies       = 5  // ICoreWebView2CookieManager::GetCookies
	slotListCount        = 3  // ICoreWebView2CookieList::get_Count
	slotListItem         = 4  // ICoreWebView2CookieList::GetValueAtIndex
	slotCookieName       = 3  // ICoreWebView2Cookie::get_Name
	slotCookieValue      = 4  // ICoreWebView2Cookie::get_Value
	slotCookieDomain     = 6  // ICoreWebView2Cookie::get_Domain
	eNoInterface         = 0x80004002
)

var (
	iidUnknown                    = windows.GUID{Data1: 0x00000000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidCoreWebView2_2             = windows.GUID{Data1: 0x9E8F0CF8, Data2: 0xE670, Data3: 0x4B5E, Data4: [8]byte{0xB2, 0xBC, 0x73, 0xE0, 0x61, 0xE3, 0x18, 0x4C}}
	iidGetCookiesCompletedHandler = windows.GUID{Data1: 0x5A4F5069, Data2: 0x5C15, Data3: 0x47C3, Data4: [8]byte{0x86, 0x46, 0xF4, 0xDE, 0x1C, 0x11, 0x66, 0x70}}
)

// comCall calls method slot of the COM object obj.
func comCall(obj unsafe.Pointer, slot int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	hr, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(obj)}, args...)...)
	return hr
}

// comString reads a string out-parameter and frees it.
func comString(obj unsafe.Pointer, slot int) string {
	var s *uint16
	if comCall(obj, slot, uintptr(unsafe.Pointer(&s))) != 0 || s == nil {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(s))
	return windows.UTF16PtrToString(s)
}

// coreWebView2 is the sign-in window's ICoreWebView2.
func coreWebView2(w *application.WebviewWindow) unsafe.Pointer {
	return webviewHandle(w, "chromium", "webview")
}

func readWebviewURL(w *application.WebviewWindow) string {
	wv := coreWebView2(w)
	if wv == nil {
		return ""
	}
	return application.InvokeSyncWithResult(func() string { return comString(wv, slotGetSource) })
}

// cookiesHandler is the ICoreWebView2GetCookiesCompletedHandler that
// GetCookies calls back, on the UI thread, with the cookie list.
type cookiesHandler struct {
	vtbl *[4]uintptr
	done chan map[string]string
}

var cookiesHandlerVtbl = &[4]uintptr{
	windows.NewCallback(func(h *cookiesHandler, iid *windows.GUID, out *unsafe.Pointer) uintptr {
		if *iid == iidUnknown || *iid == iidGetCookiesCompletedHandler {
			*out = unsafe.Pointer(h)
			return 0
		}
		*out = nil
		return eNoInterface
	}),
	windows.NewCallback(func(*cookiesHandler) uintptr { return 1 }), // AddRef: kept alive by pendingCookies
	windows.NewCallback(func(*cookiesHandler) uintptr { return 1 }), // Release
	windows.NewCallback(func(h *cookiesHandler, hr uintptr, list unsafe.Pointer) uintptr {
		cookies := map[string]string{}
		var count uint32
		if hr == 0 && list != nil && comCall(list, slotListCount, uintptr(unsafe.Pointer(&count))) == 0 {
			for i := uint32(0); i < count; i++ {
				var cookie unsafe.Pointer
				if comCall(list, slotListItem, uintptr(i), uintptr(unsafe.Pointer(&cookie))) != 0 || cookie == nil {
					continue
				}
				name, value := comString(cookie, slotCookieName), comString(cookie, slotCookieValue)
				if keepGoogleCookie(name, value, comString(cookie, slotCookieDomain)) {
					cookies[name] = value
				}
				comCall(cookie, slotRelease)
			}
		}
		h.done <- cookies
		pendingCookies.Delete(h)
		return 0
	}),
}

// pendingCookies keeps handlers WebView2 still holds out of the GC's reach.
var pendingCookies sync.Map

func readWebviewGoogleCookies(w *application.WebviewWindow) (map[string]string, error) {
	wv := coreWebView2(w)
	if wv == nil {
		return nil, errors.New("Google sign-in window is closed. Open it again")
	}
	h := &cookiesHandler{vtbl: cookiesHandlerVtbl, done: make(chan map[string]string, 1)}
	pendingCookies.Store(h, struct{}{})
	hr := application.InvokeSyncWithResult(func() uintptr {
		var wv2, manager unsafe.Pointer
		if hr := comCall(wv, slotQueryInterface, uintptr(unsafe.Pointer(&iidCoreWebView2_2)), uintptr(unsafe.Pointer(&wv2))); hr != 0 {
			return hr
		}
		defer comCall(wv2, slotRelease)
		if hr := comCall(wv2, slotGetCookieManager, uintptr(unsafe.Pointer(&manager))); hr != 0 {
			return hr
		}
		defer comCall(manager, slotRelease)
		uri, _ := windows.UTF16PtrFromString("https://messages.google.com/web/config")
		return comCall(manager, slotGetCookies, uintptr(unsafe.Pointer(uri)), uintptr(unsafe.Pointer(h)))
	})
	if hr != 0 {
		pendingCookies.Delete(h)
		return nil, errors.New("could not read cookies from the Google sign-in window. Update Microsoft Edge WebView2 and try again")
	}
	select {
	case cookies := <-h.done:
		return checkGoogleCookies(cookies)
	case <-time.After(10 * time.Second):
		return nil, errors.New("could not read cookies from the Google sign-in window")
	}
}
