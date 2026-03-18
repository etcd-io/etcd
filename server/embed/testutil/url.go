package testutil

import (
	"fmt"
	"net/url"
	"os"
)

type ConfigTestURLs struct {
	PeerURLs       []url.URL
	ClientURLs     []url.URL
	InitialCluster string
}

// Similar to function in integration/embed/embed_test.go for setting up Config.
func NewConfigTestURLs() *ConfigTestURLs {
	urls := newEmbedURLs(2)
	cfg := &ConfigTestURLs{
		ClientURLs:     []url.URL{urls[0]},
		PeerURLs:       []url.URL{urls[1]},
		InitialCluster: "",
	}
	for i := range cfg.PeerURLs {
		cfg.InitialCluster += ",default=" + cfg.PeerURLs[i].String()
	}
	cfg.InitialCluster = cfg.InitialCluster[1:]
	return cfg
}

func newEmbedURLs(n int) (urls []url.URL) {
	scheme := "unix"
	for i := 0; i < n; i++ {
		u, _ := url.Parse(fmt.Sprintf("%s://localhost:%d%06d", scheme, os.Getpid(), i))
		urls = append(urls, *u)
	}
	return urls
}
