package main

import (
	"fmt"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"time"
)

var (
	gistCache         = expirable.NewLRU[string, *ResponseCache](128, nil, time.Minute*1)
	githubassetsCache = expirable.NewLRU[string, *ResponseCache](16, nil, time.Minute*30)
	githubrawCache    = expirable.NewLRU[string, *ResponseCache](128, nil, time.Minute*1)
	douyuCache        = expirable.NewLRU[string, *ResponseCache](128, nil, time.Second*5)
)

func init() {
	go Monitor()
}

func Monitor() {
	for {
		select {
		case <-time.After(time.Second * 30):
			fmt.Printf("Cache monitor /gist/%d/githubassets/%d/githubraw/%d/douyu/%d/\n", gistCache.Len(), githubassetsCache.Len(), githubrawCache.Len(), douyuCache.Len())
		}
	}
}
