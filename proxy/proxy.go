package proxy

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func NewHandler(endpoints []string) (*httputil.ReverseProxy, error) {
	d, err := newDirector(endpoints)
	if err != nil {
		return nil, err
	}

	proxy := httputil.ReverseProxy{
		Director:      d.direct,
		Transport:     &http.Transport{},
		FlushInterval: 0,
	}

	return &proxy, nil
}

func newDirector(endpoints []string) (*director, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("one or more endpoints required")
	}

	urls := make([]url.URL, len(endpoints))
	for i, e := range endpoints {
		u, err := url.Parse(e)
		if err != nil {
			return nil, fmt.Errorf("invalid endpoint %q: %v", e, err)
		}

		if u.Scheme == "" {
			return nil, fmt.Errorf("invalid endpoint %q: scheme required", e)
		}

		if u.Host == "" {
			return nil, fmt.Errorf("invalid endpoint %q: host empty", e)
		}

		urls[i] = *u
	}

	d := director{
		endpoints: urls,
	}

	return &d, nil
}

type director struct {
	endpoints []url.URL
}

func (d *director) direct(req *http.Request) {
	choice := d.endpoints[0]
	req.URL.Scheme = choice.Scheme
	req.URL.Host = choice.Host
}
