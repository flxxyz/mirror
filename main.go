package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed index.html
var indexHTML []byte

const (
	userAgent               = "Mirror (+https://github.com/flxxyz/mirror)"
	originalGistURL         = "https://gist.github.com/"
	originalGithubAssetsURL = "https://github.githubassets.com/"
	originalDouyuURL        = "https://open.douyucdn.cn/"
)

var (
	useProxy bool
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

	r.Use(middleware.Throttle(1000))
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestSize(1 << 10)) // 1 KB request size limit
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.NoCache)
	r.Use(middleware.Compress(5, "gzip", "deflate", "br"))
	r.Use(middleware.Recoverer)

	//r.Mount("/debug", middleware.Profiler())

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	r.Route("/gist", func(r chi.Router) {
		r.Get("/{username}/*", func(w http.ResponseWriter, r *http.Request) {
			username := chi.URLParam(r, "username")
			filename := filepath.Base(r.RequestURI) // Get the filename from the request URI

			uri, _ := url.Parse(originalGistURL)
			uri.Path = filepath.Join(username, filename)

			response(r.Context(), uri, w, func(buf *bytes.Buffer) {
				bodyString := strings.Replace(buf.String(), originalGithubAssetsURL, getOriginalURL(r).String(), -1)
				buf.Truncate(0)
				buf.WriteString(bodyString)
			})
		})
	})

	r.Route("/githubassets", func(r chi.Router) {
		r.Get("/{src}/*", func(w http.ResponseWriter, r *http.Request) {
			srcDir := chi.URLParam(r, "src")
			filename := filepath.Base(r.RequestURI) // Get the filename from the request URI

			uri, _ := url.Parse(originalGithubAssetsURL)
			uri.Path = filepath.Join(srcDir, filename)

			response(r.Context(), uri, w)
		})
	})

	r.Route("/douyu", func(r chi.Router) {
		r.Get("/api/RoomApi/room/{roomid}", func(w http.ResponseWriter, r *http.Request) {
			roomID := chi.URLParam(r, "roomid")

			uri, _ := url.Parse(originalDouyuURL)
			uri.Path = fmt.Sprintf("/api/RoomApi/room/%s", roomID)

			response(r.Context(), uri, w)
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
func response(ctx context.Context, uri *url.URL, w http.ResponseWriter, hooks ...func(*bytes.Buffer)) {
	contentType, body, err := request(ctx, uri)
	if err != nil {
		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		return
	}

	buf := bytes.NewBuffer(body)

	for _, hook := range hooks {
		if hook != nil {
			hook(buf)
		}
	}

	w.Header().Set("Content-Type", contentType)
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
	if r.TLS != nil {
		scheme = "https"
	}
	return &url.URL{
		Scheme: scheme,
		Host:   r.Host,
	}
}
