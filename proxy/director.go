// Copyright 2015 CoreOS, Inc.
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

package proxy

import (
	"log"
	"math/rand"
	"net/url"
	"sync"
	"time"
)

func newDirector(urlsFunc GetProxyURLs, failureWait time.Duration, refreshInterval time.Duration) *director {
	d := &director{
		uf:              urlsFunc,
		failureWait:     failureWait,
		refreshInterval: refreshInterval,
	}
	d.refresh()
	go func() {
		for {
			select {
			case <-time.After(refreshInterval):
				d.refresh()
			}
		}
	}()
	return d
}

type director struct {
	sync.Mutex
	ep              []*endpoint
	uf              GetProxyURLs
	failureWait     time.Duration
	refreshInterval time.Duration
}

func (d *director) refresh() {
	urls := d.uf()
	d.Lock()
	defer d.Unlock()
	var endpoints []*endpoint
	for _, u := range urls {
		uu, err := url.Parse(u)
		if err != nil {
			log.Printf("proxy: upstream URL invalid: %v", err)
			continue
		}
		endpoints = append(endpoints, newEndpoint(*uu, d.failureWait))
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

func newEndpoint(u url.URL, failureWait time.Duration) *endpoint {
	ep := endpoint{
		URL:       u,
		Available: true,
		failFunc:  timedUnavailabilityFunc(failureWait),
	}

	return &ep
}

type endpoint struct {
	sync.Mutex

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

	log.Printf("proxy: marked endpoint %s unavailable", ep.URL.String())

	if ep.failFunc == nil {
		log.Printf("proxy: no failFunc defined, endpoint %s will be unavailable forever.", ep.URL.String())
		return
	}

	ep.failFunc(ep)
}

func timedUnavailabilityFunc(wait time.Duration) func(*endpoint) {
	return func(ep *endpoint) {
		time.AfterFunc(wait, func() {
			ep.Available = true
			log.Printf("proxy: marked endpoint %s available", ep.URL.String())
		})
	}
}
