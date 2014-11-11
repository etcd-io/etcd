/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/etcd/etcdserver/etcdhttp/httptypes"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
// This list of headers borrowed from stdlib httputil.ReverseProxy
var singleHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func removeSingleHopHeaders(hdrs *http.Header) {
	for _, h := range singleHopHeaders {
		hdrs.Del(h)
	}
}

type reverseProxy struct {
	director  *director
	transport http.RoundTripper
}

func (p *reverseProxy) ServeHTTP(rw http.ResponseWriter, clientreq *http.Request) {
	proxyreq := new(http.Request)
	*proxyreq = *clientreq

	// deep-copy the headers, as these will be modified below
	proxyreq.Header = make(http.Header)
	copyHeader(proxyreq.Header, clientreq.Header)

	normalizeRequest(proxyreq)
	removeSingleHopHeaders(&proxyreq.Header)
	maybeSetForwardedFor(proxyreq)

	endpoints := p.director.endpoints()
	if len(endpoints) == 0 {
		msg := "proxy: zero endpoints currently available"
		// TODO: limit the rate of the error logging.
		log.Printf(msg)
		e := httptypes.NewHTTPError(http.StatusServiceUnavailable, msg)
		e.WriteTo(rw)
		return
	}

	var res *http.Response
	var err error

	for _, ep := range endpoints {
		redirectRequest(proxyreq, ep.URL)

		res, err = p.transport.RoundTrip(proxyreq)
		if err != nil {
			log.Printf("proxy: failed to direct request to %s: %v", ep.URL.String(), err)
			ep.Failed()
			continue
		}

		break
	}

	if res == nil {
		// TODO: limit the rate of the error logging.
		msg := fmt.Sprintf("proxy: unable to get response from %d endpoint(s)", len(endpoints))
		log.Printf(msg)
		e := httptypes.NewHTTPError(http.StatusBadGateway, msg)
		e.WriteTo(rw)
		return
	}

	defer res.Body.Close()

	removeSingleHopHeaders(&res.Header)
	copyHeader(rw.Header(), res.Header)

	rw.WriteHeader(res.StatusCode)
	io.Copy(rw, res.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func redirectRequest(req *http.Request, loc url.URL) {
	req.URL.Scheme = loc.Scheme
	req.URL.Host = loc.Host
}

func normalizeRequest(req *http.Request) {
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1
	req.Close = false
}

func maybeSetForwardedFor(req *http.Request) {
	clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return
	}

	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := req.Header["X-Forwarded-For"]; ok {
		clientIP = strings.Join(prior, ", ") + ", " + clientIP
	}
	req.Header.Set("X-Forwarded-For", clientIP)
}
