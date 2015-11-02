// Copyright 2014 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.  See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.
//
// Author: Tyler Neely (t@jujit.su)

package loghisto

import (
	"net"
	"sync"
	"time"

)

type requestable interface{}

type requestableArray interface {
	ToRequest() []byte
}

// Submitter encapsulates the state of a metric submitter.
type Submitter struct {
	// backlog works as an evicting queue
	backlog            [60][]byte
	backlogHead        uint
	backlogTail        uint
	backlogMu          sync.Mutex
	serializer         func(*ProcessedMetricSet) []byte
	DestinationNetwork string
	DestinationAddress string
	metricSystem       *MetricSystem
	metricChan         chan *ProcessedMetricSet
	shutdownChan       chan struct{}
}

// NewSubmitter creates a Submitter that receives metrics off of a
// specified metric channel, serializes them using the provided
// serialization function, and attempts to send them to the
// specified destination.
func NewSubmitter(metricSystem *MetricSystem,
	serializer func(*ProcessedMetricSet) []byte, destinationNetwork string,
	destinationAddress string) *Submitter {
	metricChan := make(chan *ProcessedMetricSet, 60)
	metricSystem.SubscribeToProcessedMetrics(metricChan)
	return &Submitter{
		backlog:            [60][]byte{},
		backlogHead:        0,
		backlogTail:        0,
		serializer:         serializer,
		DestinationNetwork: destinationNetwork,
		DestinationAddress: destinationAddress,
		metricSystem:       metricSystem,
		metricChan:         metricChan,
		shutdownChan:       make(chan struct{}),
	}
}

func (s *Submitter) retryBacklog() error {
	var request []byte
	for {
		s.backlogMu.Lock()
		head := s.backlogHead
		tail := s.backlogTail
		if head != tail {
			request = s.backlog[head]
		}
		s.backlogMu.Unlock()

		if head == tail {
			return nil
		}

		err := s.submit(request)
		if err != nil {
			return err
		}
		s.backlogMu.Lock()
		s.backlogHead = (s.backlogHead + 1) % 60
		s.backlogMu.Unlock()
	}
}

func (s *Submitter) appendToBacklog(request []byte) {
	s.backlogMu.Lock()
	s.backlog[s.backlogTail] = request
	s.backlogTail = (s.backlogTail + 1) % 60
	// if we've run into the head, evict it
	if s.backlogHead == s.backlogTail {
		s.backlogHead = (s.backlogHead + 1) % 60
	}
	s.backlogMu.Unlock()
}

func (s *Submitter) submit(request []byte) error {
	conn, err := net.DialTimeout(s.DestinationNetwork, s.DestinationAddress,
		5*time.Second)
	if err != nil {
		return err
	}
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(request)
	conn.Close()
	return err
}

// Start creates the goroutines that receive, serialize, and send metrics.
func (s *Submitter) Start() {
	go func() {
		for {
			select {
			case metrics, ok := <-s.metricChan:
				if !ok {
					// We can no longer make progress.
					return
				}
				request := s.serializer(metrics)
				s.appendToBacklog(request)
			case <-s.shutdownChan:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-s.shutdownChan:
				return
			default:
				s.retryBacklog()
				tts := s.metricSystem.interval.Nanoseconds() -
					(time.Now().UnixNano() % s.metricSystem.interval.Nanoseconds())
				time.Sleep(time.Duration(tts))
			}
		}
	}()
}

// Shutdown shuts down a submitter
func (s *Submitter) Shutdown() {
	select {
	case <-s.shutdownChan:
		// already closed
	default:
		close(s.shutdownChan)
	}
}
