package main

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const (
	SchemeHTTP  = "http"
	SchemeHTTPS = "https"
)

// validateURL checks if the provided string is a valid URL.
func validateURL(s string) bool {
	if s == "" {
		return false
	}

	_, err := url.Parse(s)
	if err != nil {
		// If the URL is invalid, print an error message and return false
		return false
	}

	return true
}

func getOriginalURL(r *http.Request) *url.URL {
	scheme := SchemeHTTP

	if !slices.ContainsFunc(disallowHosts, func(s string) bool {
		return strings.HasPrefix(r.Host, s)
	}) {
		scheme = SchemeHTTPS
	}

	if r.TLS != nil {
		scheme = SchemeHTTPS
	}

	if r.Referer() != "" {
		referer, _ := url.Parse(r.Referer())
		scheme = referer.Scheme
	}

	if s, ok := r.Header["X-Forwarded-Proto"]; ok {
		if len(s) >= 1 {
			scheme = s[0]
		}
	}

	return &url.URL{
		Scheme: scheme,
		Host:   r.Host,
	}
}
