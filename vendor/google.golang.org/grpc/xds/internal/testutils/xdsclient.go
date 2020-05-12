/*
 *
 * Copyright 2019 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package testutils

import (
	xdsclient "google.golang.org/grpc/xds/internal/client"
)

// XDSClient is a fake implementation of an xds client. It exposes a bunch of
// channels to signal the occurrence of various events.
type XDSClient struct {
	serviceCb  func(xdsclient.ServiceUpdate, error)
	suWatchCh  *Channel
	closeCh    *Channel
	suCancelCh *Channel
}

// WatchService registers a LDS/RDS watch.
func (xdsC *XDSClient) WatchService(target string, callback func(xdsclient.ServiceUpdate, error)) func() {
	xdsC.serviceCb = callback
	xdsC.suWatchCh.Send(target)
	return func() {
		xdsC.suCancelCh.Send(nil)
	}
}

// WaitForWatchService waits for WatchService to be invoked on this client
// within a reasonable timeout.
func (xdsC *XDSClient) WaitForWatchService() (string, error) {
	val, err := xdsC.suWatchCh.Receive()
	return val.(string), err
}

// InvokeWatchServiceCb invokes the registered service watch callback.
func (xdsC *XDSClient) InvokeWatchServiceCb(cluster string, err error) {
	xdsC.serviceCb(xdsclient.ServiceUpdate{Cluster: cluster}, err)
}

// Close closes the xds client.
func (xdsC *XDSClient) Close() {
	xdsC.closeCh.Send(nil)
}

// NewXDSClient returns a new fake xds client.
func NewXDSClient() *XDSClient {
	return &XDSClient{
		suWatchCh:  NewChannel(),
		closeCh:    NewChannel(),
		suCancelCh: NewChannel(),
	}
}
