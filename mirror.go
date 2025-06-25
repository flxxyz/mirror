package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
)

var (
	HTTPHeaderContentType   = "Content-Type"
	HTTPHeaderContentLength = "Content-Length"
	HTTPHeaderUserAgent     = "User-Agent"
	HTTPHeaderMirrorCache   = "X-Mirror-Cache"

	HTTPResponseCacheMISS = "MISS"
	HTTPResponseCacheHIT  = "HIT"
)

type Mirror struct {
	Uri           *url.URL
	Cache         *expirable.LRU[string, *ResponseCache]
	BeforeHooks   []func(ctx context.Context, m *Mirror)
	AfterHooks    []func(ctx context.Context, m *Mirror)
	ContentType   string
	Body          *bytes.Buffer
	ContentLength string
}

func (m *Mirror) Response(ctx context.Context, w http.ResponseWriter) {
	// directly return if the request is cache
	if val, ok := m.Cache.Get(m.Uri.String()); ok {
		w.Header().Set(HTTPHeaderMirrorCache, HTTPResponseCacheHIT)
		w.Header().Set(HTTPHeaderContentType, val.ContentType)
		w.Header().Set(HTTPHeaderContentLength, m.ContentLength)
		_, _ = w.Write(val.Body)
		return
	}

	for _, hook := range m.BeforeHooks {
		if hook != nil {
			hook(ctx, m)
		}
	}

	if err := m.Fetch(ctx, m.Uri); err != nil {
		if err.Error() == fmt.Sprintf("%d", http.StatusNotFound) {
			http.Error(w, "Resource not found", http.StatusNotFound)
			return
		}

		if errors.Is(err, context.Canceled) {
			http.Error(w, "Timeout", http.StatusInternalServerError)
			return
		}

		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		return
	}

	for _, hook := range m.AfterHooks {
		if hook != nil {
			hook(ctx, m)
		}
	}

	// cache the response
	lruKey := m.Uri.String()
	if !m.Cache.Contains(lruKey) {
		m.Cache.Add(lruKey, &ResponseCache{
			ContentType: m.ContentType,
			Body:        m.Body.Bytes(),
		})
	}

	w.Header().Set(HTTPHeaderMirrorCache, HTTPResponseCacheMISS)
	w.Header().Set(HTTPHeaderContentType, m.ContentType)
	w.Header().Set(HTTPHeaderContentLength, m.ContentLength)
	_, err := w.Write(m.Body.Bytes())
	if err != nil {
		m.Cache.Remove(lruKey) // Remove from cache if write fails
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// Fetch fetches the content from the given URI.
func (m *Mirror) Fetch(ctx context.Context, uri *url.URL) error {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", addr)
		},
	}

	// 配置代理服务器
	if useProxy {
		proxyURL, err := url.Parse(os.Getenv("HTTP_PROXY"))
		if err != nil {
			return nil
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := http.Client{
		Transport: transport,
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, uri.String(), nil)

	req.Header.Set(HTTPHeaderUserAgent, userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Process the body as needed
	m.Body = bytes.NewBuffer(body)
	m.ContentType = resp.Header.Get(HTTPHeaderContentType)
	m.ContentLength = resp.Header.Get(HTTPHeaderContentLength)

	return nil
}
