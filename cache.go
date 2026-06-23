package main

import (
	"fmt"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"os"
	"time"
)

// cacheTTL reads CACHE_<name> (e.g. "5s", "30m"); falls back to def if unset/invalid.
func cacheTTL(name string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(os.Getenv("CACHE_" + name)); err == nil {
		return d
	}
	return def
}

var (
	gistCache         = expirable.NewLRU[string, *ResponseCache](128, nil, cacheTTL("GIST", time.Minute*1))
	githubassetsCache = expirable.NewLRU[string, *ResponseCache](16, nil, cacheTTL("GITHUBASSETS", time.Minute*30))
	githubrawCache    = expirable.NewLRU[string, *ResponseCache](128, nil, cacheTTL("GITHUBRAW", time.Minute*1))
	douyuCache        = expirable.NewLRU[string, *ResponseCache](128, nil, cacheTTL("DOUYU", time.Second*5))
	directCache       = expirable.NewLRU[string, *ResponseCache](16, nil, cacheTTL("DIRECT", time.Minute*1))
)

func init() {
	go Monitor()
}

func Monitor() {
	for {
		select {
		case <-time.After(time.Second * 30):
			fmt.Printf("Cache monitor /gist/%d/githubassets/%d/githubraw/%d/douyu/%d/direct/%d\n", gistCache.Len(), githubassetsCache.Len(), githubrawCache.Len(), douyuCache.Len(), directCache.Len())
		}
	}
}
