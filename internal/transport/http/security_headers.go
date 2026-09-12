package http

import (
	"net/http"
	"strings"
)

// appCSP is the Content-Security-Policy for the app document.
// This is the OUTER
// SPA's policy — the reader iframe keeps its own stricter CSP, untouched.
//
//   - default-src 'self' — everything the SPA loads is same-origin.
//   - no 'unsafe-inline' on script-src: the Vite build emits no inline
//     script.
//   - style-src allows 'unsafe-inline' for dynamic runtime style attributes
//     and elements inserted by React/Radix UI.
//   - connect-src 'self' — the API is same-origin; a reverse-proxy
//     deployment on another hostname adds it via CORS, not here.
//   - frame-ancestors 'none' + X-Frame-Options: DENY — belt and braces
//     against clickjacking of authenticated actions (the Bearer-not-cookie
//     property does NOT help here — a framed SPA attaches its own token).
var appCSP = strings.Join([]string{
	"default-src 'self'",
	"script-src 'self'",
	"style-src 'self' 'unsafe-inline'",
	"img-src 'self' data:",
	"font-src 'self'",
	"connect-src 'self'",
	"object-src 'none'",
	"base-uri 'none'",
	"frame-ancestors 'none'",
	"form-action 'self'",
}, "; ")

// permissionsPolicy denies every browser feature the app does not use.
const permissionsPolicy = "accelerometer=(), autoplay=(), camera=(), " +
	"clipboard-write=(self), display-capture=(), encrypted-media=(), " +
	"fullscreen=(self), geolocation=(), gyroscope=(), magnetometer=(), " +
	"microphone=(), midi=(), payment=(), usb=(), xr-spatial-tracking=()"

// SecurityHeaders sets the app-document security headers on EVERY response
// on every bind — plaintext included, since clickjacking and
// content-sniffing do not require TLS. It runs just inside logging
// so it covers the SPA HTML, the SPA fallback, every /api/v1 response
// (success and error), and the rate-limiter's 429 / CORS 403. The one
// uncovered path is a request the limits layer rejects before this runs
// (an oversized body) — that response is a JSON error, not a framed HTML
// document, so it carries no clickjacking or sniffing risk.
func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", appCSP)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Permissions-Policy", permissionsPolicy)
			next.ServeHTTP(w, r)
		})
	}
}

// HSTS adds Strict-Transport-Security only when the server is actually
// serving TLS in-process — a public bind, or a private bind with an
// opt-in certificate. It MUST NOT be sent on a plaintext bind: HSTS
// asserted over plaintext, or on a bare LAN IP, is pointless and
// occasionally harmful. The decision is made once at startup from the
// listener choice, not per request — pass tlsInProcess accordingly.
//
// max-age is one year; no preload, no includeSubDomains (a self-hoster's
// subdomain layout is theirs).
func HSTS(tlsInProcess bool) Middleware {
	return func(next http.Handler) http.Handler {
		if !tlsInProcess {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
			next.ServeHTTP(w, r)
		})
	}
}
