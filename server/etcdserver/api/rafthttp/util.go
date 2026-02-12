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

package rafthttp

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/pkg/v3/types"
)

var (
	errMemberRemoved  = fmt.Errorf("the member has been permanently removed from the cluster")
	errMemberNotFound = fmt.Errorf("member not found")
)

// NewListener returns a listener for raft message transfer between peers.
// It uses timeout listener to identify broken streams promptly.
func NewListener(u url.URL, tlsinfo *transport.TLSInfo) (net.Listener, error) {
	return transport.NewListenerWithOpts(u.Host, u.Scheme, transport.WithTLSInfo(tlsinfo), transport.WithTimeout(ConnReadTimeout, ConnWriteTimeout))
}

// NewRoundTripper returns a roundTripper used to send requests
// to rafthttp listener of remote peers.
func NewRoundTripper(tlsInfo transport.TLSInfo, dialTimeout time.Duration) (http.RoundTripper, error) {
	// It uses timeout transport to pair with remote timeout listeners.
	// It sets no read/write timeout, because message in requests may
	// take long time to write out before reading out the response.
	tr, err := transport.NewTimeoutTransport(tlsInfo, dialTimeout, 0, 0)
	if err != nil {
		return nil, err
	}
	applyPeerProxyMap(tr)
	return tr, nil
}

// newStreamRoundTripper returns a roundTripper used to send stream requests
// to rafthttp listener of remote peers.
// Read/write timeout is set for stream roundTripper to promptly
// find out broken status, which minimizes the number of messages
// sent on broken connection.
func newStreamRoundTripper(tlsInfo transport.TLSInfo, dialTimeout time.Duration) (http.RoundTripper, error) {
	tr, err := transport.NewTimeoutTransport(tlsInfo, dialTimeout, ConnReadTimeout, ConnWriteTimeout)
	if err != nil {
		return nil, err
	}
	applyPeerProxyMap(tr)
	return tr, nil
}

// applyPeerProxyMap checks the ETCD_PEER_PROXY_MAP environment variable and,
// if set, wraps the transport's DialContext to redirect outbound peer
// connections through local forward proxies. This enables robustness tests
// to intercept and blackhole outbound peer traffic per member.
//
// The env var format is: "host1:port1=proxyhost1:proxyport1,host2:port2=proxyhost2:proxyport2"
// When the transport dials a destination matching a key, it connects to the
// corresponding proxy address instead.
func applyPeerProxyMap(tr *http.Transport) {
	proxyMapStr := os.Getenv("ETCD_PEER_PROXY_MAP")
	if proxyMapStr == "" {
		return
	}
	proxyMap := parsePeerProxyMap(proxyMapStr)
	if len(proxyMap) == 0 {
		return
	}
	origDialContext := tr.DialContext
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if proxyAddr, ok := proxyMap[addr]; ok {
			addr = proxyAddr
		}
		return origDialContext(ctx, network, addr)
	}
}

// parsePeerProxyMap parses the ETCD_PEER_PROXY_MAP format:
// "dest1=proxy1,dest2=proxy2"
func parsePeerProxyMap(s string) map[string]string {
	m := make(map[string]string)
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		dest := strings.TrimSpace(parts[0])
		proxy := strings.TrimSpace(parts[1])
		if dest != "" && proxy != "" {
			m[dest] = proxy
		}
	}
	return m
}

// createPostRequest creates a HTTP POST request that sends raft message.
func createPostRequest(lg *zap.Logger, u url.URL, path string, body io.Reader, ct string, urls types.URLs, from, cid types.ID) *http.Request {
	uu := u
	uu.Path = path
	req, err := http.NewRequest(http.MethodPost, uu.String(), body)
	if err != nil {
		if lg != nil {
			lg.Panic("unexpected new request error", zap.Error(err))
		}
	}
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-Server-From", from.String())
	req.Header.Set("X-Server-Version", version.Version)
	req.Header.Set("X-Min-Cluster-Version", version.MinClusterVersion)
	req.Header.Set("X-Etcd-Cluster-ID", cid.String())
	setPeerURLsHeader(req, urls)

	return req
}

// checkPostResponse checks the response of the HTTP POST request that sends
// raft message.
func checkPostResponse(lg *zap.Logger, resp *http.Response, body []byte, req *http.Request, to types.ID) error {
	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		switch strings.TrimSuffix(string(body), "\n") {
		case errIncompatibleVersion.Error():
			if lg != nil {
				lg.Error(
					"request sent was ignored by peer",
					zap.String("remote-peer-id", to.String()),
				)
			}
			return errIncompatibleVersion
		case ErrClusterIDMismatch.Error():
			if lg != nil {
				lg.Error(
					"request sent was ignored due to cluster ID mismatch",
					zap.String("remote-peer-id", to.String()),
					zap.String("remote-peer-cluster-id", resp.Header.Get("X-Etcd-Cluster-ID")),
					zap.String("local-member-cluster-id", req.Header.Get("X-Etcd-Cluster-ID")),
				)
			}
			return ErrClusterIDMismatch
		default:
			return fmt.Errorf("unhandled error %q when precondition failed", string(body))
		}
	case http.StatusForbidden:
		return errMemberRemoved
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("unexpected http status %s while posting to %q", http.StatusText(resp.StatusCode), req.URL.String())
	}
}

// reportCriticalError reports the given error through sending it into
// the given error channel.
// If the error channel is filled up when sending error, it drops the error
// because the fact that error has happened is reported, which is
// good enough.
func reportCriticalError(err error, errc chan<- error) {
	select {
	case errc <- err:
	default:
	}
}

// compareMajorMinorVersion returns an integer comparing two versions based on
// their major and minor version. The result will be 0 if a==b, -1 if a < b,
// and 1 if a > b.
func compareMajorMinorVersion(a, b *semver.Version) int {
	na := &semver.Version{Major: a.Major, Minor: a.Minor}
	nb := &semver.Version{Major: b.Major, Minor: b.Minor}
	switch {
	case na.LessThan(*nb):
		return -1
	case nb.LessThan(*na):
		return 1
	default:
		return 0
	}
}

// serverVersion returns the server version from the given header.
func serverVersion(h http.Header) *semver.Version {
	verStr := h.Get("X-Server-Version")
	// backward compatibility with etcd 2.0
	if verStr == "" {
		verStr = "2.0.0"
	}
	return semver.Must(semver.NewVersion(verStr))
}

// serverVersion returns the min cluster version from the given header.
func minClusterVersion(h http.Header) *semver.Version {
	verStr := h.Get("X-Min-Cluster-Version")
	// backward compatibility with etcd 2.0
	if verStr == "" {
		verStr = "2.0.0"
	}
	return semver.Must(semver.NewVersion(verStr))
}

// checkVersionCompatibility checks whether the given version is compatible
// with the local version.
func checkVersionCompatibility(name string, server, minCluster *semver.Version) (
	localServer *semver.Version,
	localMinCluster *semver.Version,
	err error,
) {
	localServer = semver.Must(semver.NewVersion(version.Version))
	localMinCluster = semver.Must(semver.NewVersion(version.MinClusterVersion))
	if compareMajorMinorVersion(server, localMinCluster) == -1 {
		return localServer, localMinCluster, fmt.Errorf("remote version is too low: remote[%s]=%s, local=%s", name, server, localServer)
	}
	if compareMajorMinorVersion(minCluster, localServer) == 1 {
		return localServer, localMinCluster, fmt.Errorf("local version is too low: remote[%s]=%s, local=%s", name, server, localServer)
	}
	return localServer, localMinCluster, nil
}

// setPeerURLsHeader reports local urls for peer discovery
func setPeerURLsHeader(req *http.Request, urls types.URLs) {
	if urls == nil {
		// often not set in unit tests
		return
	}
	peerURLs := make([]string, urls.Len())
	for i := range urls {
		peerURLs[i] = urls[i].String()
	}
	req.Header.Set("X-PeerURLs", strings.Join(peerURLs, ","))
}

// addRemoteFromRequest adds a remote peer according to an http request header
func addRemoteFromRequest(tr Transporter, r *http.Request) {
	if from, err := types.IDFromString(r.Header.Get("X-Server-From")); err == nil {
		if urls := r.Header.Get("X-PeerURLs"); urls != "" {
			tr.AddRemote(from, strings.Split(urls, ","))
		}
	}
}
