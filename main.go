package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/go-chi/cors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

//go:embed index.html
var indexHTML []byte

const (
	userAgent               = "Mirror (+https://github.com/flxxyz/mirror)"
	originalGistURL         = "https://gist.github.com/"
	originalGithubAssetsURL = "https://github.githubassets.com/"
	originalDouyuURL        = "https://open.douyucdn.cn/"
)

type ResponseCache struct {
	ContentType string
	Body        []byte
}

var (
	useProxy      bool
	disallowHosts = []string{
		"localhost",
		"192.168.",
		"172.16.",
		"172.17.",
		"172.18.",
		"172.19.",
		"172.20.",
		"172.21.",
		"172.22.",
		"172.23.",
		"172.24.",
		"172.25.",
		"172.26.",
		"172.27.",
		"172.28.",
		"172.29.",
		"172.30.",
		"172.31.",
		"10.",
		"127.",
	}
	gistCache         = expirable.NewLRU[string, *ResponseCache](512, nil, time.Minute*1)
	githubassetsCache = expirable.NewLRU[string, *ResponseCache](16, nil, time.Minute*30)
	douyuCache        = expirable.NewLRU[string, *ResponseCache](512, nil, time.Second*1)
)

func init() {
	n := runtime.NumCPU()
	if n > 1 {
		n -= 1
	}
	runtime.GOMAXPROCS(n) // Set GOMAXPROCS to one less than the number of CPUs

	if validateURL(os.Getenv("HTTP_PROXY")) {
		useProxy = true
	}
}

func main() {
	r := chi.NewRouter()

	//r.Use(middleware.Heartbeat("/heartbeat"))
	//r.Use(middleware.Throttle(1000))
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestSize(1 << 10)) // 1 KB request size limit
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Compress(5, "image/jpeg", "image/png", "text/css", "text/javascript", "text/html", "application/json", "application/xml"))
	r.Use(middleware.Recoverer)
	r.Use(cors.AllowAll().Handler)
	r.Use(func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set the Cache-Control header to allow caching for 24 hour
			w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
			// set cross-origin-resource-policy
			w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
			handler.ServeHTTP(w, r)
		})
	})

	//r.Mount("/debug", middleware.Profiler())

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	r.Route("/gist", func(r chi.Router) {
		r.Get("/{username}/{filename}", func(w http.ResponseWriter, r *http.Request) {
			username := chi.URLParam(r, "username")
			filename := chi.URLParam(r, "filename")

			// disable not ".js" extname
			fileExt := filepath.Ext(filename)
			if fileExt != ".js" {
				http.Error(w, "Not allowed", http.StatusForbidden)
				return
			}

			uri, _ := url.Parse(originalGistURL)
			uri.Path = filepath.Join(username, filename)

			if val, ok := gistCache.Get(uri.String()); ok {
				w.Header().Set("Content-Type", val.ContentType)
				w.Write(val.Body)
				return
			}

			response(r.Context(), uri, w,
				func(contentType string, buf *bytes.Buffer) {
					uri := getOriginalURL(r)
					uri.Path = "/githubassets/"
					bodyString := strings.Replace(buf.String(), originalGithubAssetsURL, uri.String(), -1)
					buf.Truncate(0)
					buf.WriteString(bodyString)
				},
				func(contentType string, buf *bytes.Buffer) {
					lruKey := uri.String()
					if !gistCache.Contains(lruKey) {
						gistCache.Add(lruKey, &ResponseCache{
							ContentType: contentType,
							Body:        buf.Bytes(),
						})
					}
				})
		})
	})

	r.Route("/githubassets", func(r chi.Router) {
		r.Get("/{src}/*", func(w http.ResponseWriter, r *http.Request) {
			srcDir := chi.URLParam(r, "src")
			filename := filepath.Base(r.RequestURI) // Get the filename from the request URI

			uri, _ := url.Parse(originalGithubAssetsURL)
			uri.Path = filepath.Join(srcDir, filename)

			if val, ok := githubassetsCache.Get(uri.String()); ok {
				w.Header().Set("Content-Type", val.ContentType)
				w.Write(val.Body)
				return
			}

			response(r.Context(), uri, w,
				func(contentType string, buf *bytes.Buffer) {
					lruKey := uri.String()
					if !githubassetsCache.Contains(lruKey) {
						githubassetsCache.Add(lruKey, &ResponseCache{
							ContentType: contentType,
							Body:        buf.Bytes(),
						})
					}
				})
		})
	})

	r.Route("/douyu", func(r chi.Router) {
		r.Get("/api/RoomApi/room/{roomID}", func(w http.ResponseWriter, r *http.Request) {
			roomID := chi.URLParam(r, "roomID")

			uri, _ := url.Parse(originalDouyuURL)
			uri.Path = fmt.Sprintf("/api/RoomApi/room/%s", roomID)

			if val, ok := douyuCache.Get(uri.String()); ok {
				w.Header().Set("Content-Type", val.ContentType)
				w.Write(val.Body)
				return
			}

			response(r.Context(), uri, w,
				func(contentType string, buf *bytes.Buffer) {
					lruKey := uri.String()
					if !douyuCache.Contains(lruKey) {
						douyuCache.Add(lruKey, &ResponseCache{
							ContentType: contentType,
							Body:        buf.Bytes(),
						})
					}
				})
		})
		r.Get("//api/RoomApi/room/{roomid}", func(w http.ResponseWriter, r *http.Request) {
			roomID := chi.URLParam(r, "roomid")
			uri := getOriginalURL(r)
			uri.Path = fmt.Sprintf("/douyu/api/RoomApi/room/%s", roomID)
			w.Header().Set("Location", uri.String())
			w.WriteHeader(http.StatusPermanentRedirect)
		})
	})

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0" // Default host
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9000" // Default port
	}
	addr := fmt.Sprintf("%s:%s", host, port)
	fmt.Println("Server is running on", addr)

	// Start the HTTP server
	if err := http.ListenAndServe(addr, r); err != nil {
		panic(err)
	}
}

// request fetches the content from the given URI and returns the content type and body.
func request(ctx context.Context, uri *url.URL) (string, []byte, error) {
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
			return "", nil, nil
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := http.Client{
		Transport: transport,
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, uri.String(), nil)

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, errors.New("Failed to fetch the resource: " + resp.Status)
	}

	// Process the body as needed
	contentType := resp.Header.Get("Content-Type")

	return contentType, body, nil
}

// response handles the HTTP response for the given URI.
func response(ctx context.Context, uri *url.URL, w http.ResponseWriter, hooks ...func(contentType string, buf *bytes.Buffer)) {
	contentType, body, err := request(ctx, uri)
	if err != nil {
		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		return
	}

	buf := bytes.NewBuffer(body)

	for _, hook := range hooks {
		if hook != nil {
			hook(contentType, buf)
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	_, err = w.Write(buf.Bytes())
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

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
	scheme := "http"

	if !slices.ContainsFunc(disallowHosts, func(s string) bool {
		return strings.HasPrefix(r.Host, s)
	}) {
		scheme = "https"
	}

	if r.TLS != nil {
		scheme = "https"
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
