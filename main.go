package main

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/go-chi/cors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "go.uber.org/automaxprocs"
)

//go:embed index.html
var indexHTML []byte

const (
	userAgent                    = "Mirror (+https://github.com/flxxyz/mirror)"
	originalGistURL              = "https://gist.github.com/"
	originalGithubAssetsURL      = "https://github.githubassets.com/"
	originalGithubUserContentURL = "https://raw.githubusercontent.com/"
	originalDouyuURL             = "https://open.douyucdn.cn/"
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

func RedirectRoot(prefixes ...string) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				for _, prefix := range prefixes {
					p := strings.TrimPrefix(r.URL.Path, prefix)
					if p == "/" || p == "" {
						originalURL := getOriginalURL(r)
						http.Redirect(w, r, originalURL.String(), http.StatusPermanentRedirect)
						return
					}
				}
			}

			handler.ServeHTTP(w, r)
		})
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
	r.Use(RedirectRoot("/gist", "/gistraw", "/githubassets", "/githubraw", "/douyu"))
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
		w.Header().Set(HTTPHeaderContentType, "text/html; charset=utf-8")
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

			mirror := &Mirror{
				Uri:   uri,
				Cache: gistCache,
				AfterHooks: []func(ctx context.Context, m *Mirror){
					func(ctx context.Context, m *Mirror) {
						uri := getOriginalURL(r)
						uri.Path = "/githubassets/"
						bodyString := strings.Replace(m.Body.String(), originalGithubAssetsURL, uri.String(), -1)
						m.Body.Truncate(0)
						m.Body.WriteString(bodyString)
					},
				},
			}
			mirror.Response(r.Context(), w)
		})
	})

	r.Route("/gistraw", func(r chi.Router) {
		r.Get("/{username}/{gistID}/raw/{shaID}/{filename}", func(w http.ResponseWriter, r *http.Request) {
			username := chi.URLParam(r, "username")
			gistID := chi.URLParam(r, "gistID")
			shaID := chi.URLParam(r, "shaID")
			filename := chi.URLParam(r, "filename")

			uri, _ := url.Parse(originalGistURL)
			uri.Path = filepath.Join(username, gistID, "raw", shaID, filename)

			mirror := &Mirror{
				Uri:   uri,
				Cache: gistCache,
			}
			mirror.Response(r.Context(), w)
		})
	})

	r.Route("/githubassets", func(r chi.Router) {
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			srcDir := chi.URLParam(r, "src")
			filename := strings.TrimPrefix(r.URL.Path, "/githubassets/")

			uri, _ := url.Parse(originalGithubAssetsURL)
			uri.Path, _ = url.JoinPath(srcDir, filename)

			mirror := &Mirror{
				Uri:   uri,
				Cache: githubassetsCache,
			}
			mirror.Response(r.Context(), w)
		})
	})

	r.Route("/githubraw", func(r chi.Router) {
		r.Get("/{username}/{repo}/*", func(w http.ResponseWriter, r *http.Request) {
			username := chi.URLParam(r, "username")
			repo := chi.URLParam(r, "repo")
			filename := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/githubraw/%s", filepath.Join(username, repo)))

			uri, _ := url.Parse(originalGithubUserContentURL)
			uri.Path, _ = url.JoinPath(username, repo, filename)

			mirror := &Mirror{
				Uri:   uri,
				Cache: githubrawCache,
			}
			mirror.Response(r.Context(), w)
		})
	})

	r.Route("/douyu", func(r chi.Router) {
		r.Get("/api/RoomApi/room/{roomID}", func(w http.ResponseWriter, r *http.Request) {
			roomID := chi.URLParam(r, "roomID")

			uri, _ := url.Parse(originalDouyuURL)
			uri.Path = fmt.Sprintf("/api/RoomApi/room/%s", roomID)

			mirror := &Mirror{
				Uri:   uri,
				Cache: douyuCache,
			}
			mirror.Response(r.Context(), w)
		})
		r.Get("//api/RoomApi/room/{roomid}", func(w http.ResponseWriter, r *http.Request) {
			roomID := chi.URLParam(r, "roomid")
			uri := getOriginalURL(r)
			uri.Path = fmt.Sprintf("/douyu/api/RoomApi/room/%s", roomID)
			http.Redirect(w, r, uri.String(), http.StatusPermanentRedirect)
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
