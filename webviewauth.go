package main

import (
	"errors"
	"slices"
)

// googleCookieNames are the Google session cookies libgm pairs with; the
// first six are required.
var googleCookieNames = []string{"SID", "HSID", "SSID", "OSID", "APISID", "SAPISID", "__Secure-1PSIDTS"}

// isGoogleCookieDomain: the cookies Messages sends, and no other site's.
func isGoogleCookieDomain(domain string) bool {
	return domain == ".google.com" || domain == "google.com" || domain == ".messages.google.com" || domain == "messages.google.com"
}

// keepGoogleCookie: one of the session cookies, set for Google.
func keepGoogleCookie(name, value, domain string) bool {
	return value != "" && isGoogleCookieDomain(domain) && slices.Contains(googleCookieNames, name)
}

// checkGoogleCookies fails until the sign-in window holds every required cookie.
func checkGoogleCookies(cookies map[string]string) (map[string]string, error) {
	for _, name := range googleCookieNames[:6] {
		if cookies[name] == "" {
			return nil, errors.New("Google sign-in is still in progress. Wait for the Messages config page, then try again")
		}
	}
	return cookies, nil
}
