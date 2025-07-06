// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package httpproxy

import (
	"math/rand"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
)

// defaultRefreshInterval is the default proxyRefreshIntervalMs value
// as in etcdmain/config.go.
const defaultRefreshInterval = 30000 * time.Millisecond

var once sync.Once

func init() {
	rand.Seed(time.Now().UnixNano())
}

func newDirector(lg *zap.Logger, urlsFunc GetProxyURLs, failureWait time.Duration, refreshInterval time.Duration) *director {
	if lg == nil {
		lg = zap.NewNop()
	}
	d := &director{
		lg:          lg,
		uf:          urlsFunc,
		failureWait: failureWait,
	}
	d.refresh()
	go func() {
		// In order to prevent missing proxy endpoints in the first try:
		// when given refresh interval of defaultRefreshInterval or greater
		// and whenever there is no available proxy endpoints,
		// give 1-second refreshInterval.
		for {
			es := d.endpoints()
			ri := refreshInterval
			if ri >= defaultRefreshInterval {
				if len(es) == 0 {
					ri = time.Second
				}
			}
			if len(es) > 0 {
				once.Do(func() {
					var sl []string
					for _, e := range es {
						sl = append(sl, e.URL.String())
					}
					lg.Info("endpoints found", zap.Strings("endpoints", sl))
				})
			}
			time.Sleep(ri)
			d.refresh()
		}
	}()
	return d
}

type director struct {
	sync.Mutex
	lg          *zap.Logger
	ep          []*endpoint
	uf          GetProxyURLs
	failureWait time.Duration
}

func (d *director) refresh() {
	urls := d.uf()
	d.Lock()
	defer d.Unlock()
	var endpoints []*endpoint
	for _, u := range urls {
		uu, err := url.Parse(u)
		if err != nil {
			d.lg.Info("upstream URL invalid", zap.Error(err))
			continue
		}
		endpoints = append(endpoints, newEndpoint(d.lg, *uu, d.failureWait))
	}

	// shuffle array to avoid connections being "stuck" to a single endpoint
	for i := range endpoints {
		j := rand.Intn(i + 1)
		endpoints[i], endpoints[j] = endpoints[j], endpoints[i]
	}

	d.ep = endpoints
}

func (d *director) endpoints() []*endpoint {
	d.Lock()
	defer d.Unlock()
	filtered := make([]*endpoint, 0)
	for _, ep := range d.ep {
		if ep.Available {
			filtered = append(filtered, ep)
		}
	}

	return filtered
}

func newEndpoint(lg *zap.Logger, u url.URL, failureWait time.Duration) *endpoint {
	ep := endpoint{
		lg:        lg,
		URL:       u,
		Available: true,
		failFunc:  timedUnavailabilityFunc(failureWait),
	}

	return &ep
}

type endpoint struct {
	sync.Mutex

	lg        *zap.Logger
	URL       url.URL
	Available bool

	failFunc func(ep *endpoint)
}

func (ep *endpoint) Failed() {
	ep.Lock()
	if !ep.Available {
		ep.Unlock()
		return
	}

	ep.Available = false
	ep.Unlock()

	if ep.lg != nil {
		ep.lg.Info("marked endpoint unavailable", zap.String("endpoint", ep.URL.String()))
	}

	if ep.failFunc == nil {
		if ep.lg != nil {
			ep.lg.Info(
				"no failFunc defined, endpoint will be unavailable forever",
				zap.String("endpoint", ep.URL.String()),
			)
		}
		return
	}

	ep.failFunc(ep)
}

func timedUnavailabilityFunc(wait time.Duration) func(*endpoint) {
	return func(ep *endpoint) {
		time.AfterFunc(wait, func() {
			ep.Available = true
			if ep.lg != nil {
				ep.lg.Info(
					"marked endpoint available, to retest connectivity",
					zap.String("endpoint", ep.URL.String()),
				)
			}
		})
	}
}
