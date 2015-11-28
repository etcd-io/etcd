// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main contains a simple command line tool for Cloud Pub/Sub
// Cloud Pub/Sub docs: https://cloud.google.com/pubsub/docs
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/oauth2"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/oauth2/google"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud/compute/metadata"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud/pubsub"
)

var (
	jsonFile  = flag.String("j", "", "A path to your JSON key file for your service account downloaded from Google Developer Console, not needed if you run it on Compute Engine instances.")
	projID    = flag.String("p", "", "The ID of your Google Cloud project.")
	reportMPS = flag.Bool("report", false, "Reports the incoming/outgoing message rate in msg/sec if set.")
	size      = flag.Int("size", 10, "Batch size for pull_messages and publish_messages subcommands.")
)

const (
	usage = `Available arguments are:
    create_topic <name>
    delete_topic <name>
    create_subscription <name> <linked_topic>
    delete_subscription <name>
    publish <topic> <message>
    pull_messages <subscription> <numworkers>
    publish_messages <topic> <numworkers>
`
	tick = 1 * time.Second
)

func usageAndExit(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	fmt.Println("Flags:")
	flag.PrintDefaults()
	fmt.Fprint(os.Stderr, usage)
	os.Exit(2)
}

// Check the length of the arguments.
func checkArgs(argv []string, min int) {
	if len(argv) < min {
		usageAndExit("Missing arguments")
	}
}

// newClient creates http.Client with a jwt service account when
// jsonFile flag is specified, otherwise by obtaining the GCE service
// account's access token.
func newClient(jsonFile string) (*http.Client, error) {
	if jsonFile != "" {
		jsonKey, err := ioutil.ReadFile(jsonFile)
		if err != nil {
			return nil, err
		}
		conf, err := google.JWTConfigFromJSON(jsonKey, pubsub.ScopePubSub)
		if err != nil {
			return nil, err
		}
		return conf.Client(oauth2.NoContext), nil
	}
	if metadata.OnGCE() {
		c := &http.Client{
			Transport: &oauth2.Transport{
				Source: google.ComputeTokenSource(""),
			},
		}
		if *projID == "" {
			projectID, err := metadata.ProjectID()
			if err != nil {
				return nil, fmt.Errorf("ProjectID failed, %v", err)
			}
			*projID = projectID
		}
		return c, nil
	}
	return nil, errors.New("Could not create an authenticated client.")
}

func listTopics(ctx context.Context, argv []string) {
	panic("listTopics not implemented yet")
}

func createTopic(ctx context.Context, argv []string) {
	checkArgs(argv, 2)
	topic := argv[1]
	err := pubsub.CreateTopic(ctx, topic)
	if err != nil {
		log.Fatalf("CreateTopic failed, %v", err)
	}
	fmt.Printf("Topic %s was created.\n", topic)
}

func deleteTopic(ctx context.Context, argv []string) {
	checkArgs(argv, 2)
	topic := argv[1]
	err := pubsub.DeleteTopic(ctx, topic)
	if err != nil {
		log.Fatalf("DeleteTopic failed, %v", err)
	}
	fmt.Printf("Topic %s was deleted.\n", topic)
}

func listSubscriptions(ctx context.Context, argv []string) {
	panic("listSubscriptions not implemented yet")
}

func createSubscription(ctx context.Context, argv []string) {
	checkArgs(argv, 3)
	sub := argv[1]
	topic := argv[2]
	err := pubsub.CreateSub(ctx, sub, topic, 60*time.Second, "")
	if err != nil {
		log.Fatalf("CreateSub failed, %v", err)
	}
	fmt.Printf("Subscription %s was created.\n", sub)
}

func deleteSubscription(ctx context.Context, argv []string) {
	checkArgs(argv, 2)
	sub := argv[1]
	err := pubsub.DeleteSub(ctx, sub)
	if err != nil {
		log.Fatalf("DeleteSub failed, %v", err)
	}
	fmt.Printf("Subscription %s was deleted.\n", sub)
}

func publish(ctx context.Context, argv []string) {
	checkArgs(argv, 3)
	topic := argv[1]
	message := argv[2]
	msgIDs, err := pubsub.Publish(ctx, topic, &pubsub.Message{
		Data: []byte(message),
	})
	if err != nil {
		log.Fatalf("Publish failed, %v", err)
	}
	fmt.Printf("Message '%s' published to a topic %s and the message id is %s\n", message, topic, msgIDs[0])
}

type reporter struct {
	reportTitle string
	lastC       uint64
	c           uint64
	result      <-chan int
}

func (r *reporter) report() {
	ticker := time.NewTicker(tick)
	defer func() {
		ticker.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			n := r.c - r.lastC
			r.lastC = r.c
			mps := n / uint64(tick/time.Second)
			log.Printf("%s ~%d msgs/s, total: %d", r.reportTitle, mps, r.c)
		case n := <-r.result:
			r.c += uint64(n)
		}
	}
}

func ack(ctx context.Context, sub string, ackID ...string) {
	err := pubsub.Ack(ctx, sub, ackID...)
	if err != nil {
		log.Printf("Ack failed, %v\n", err)
	}
}

func pullLoop(ctx context.Context, sub string, result chan<- int) {
	for {
		msgs, err := pubsub.PullWait(ctx, sub, *size)
		if err != nil {
			log.Printf("PullWait failed, %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if len(msgs) == 0 {
			log.Println("Received no messages")
			continue
		}
		if *reportMPS {
			result <- len(msgs)
		}
		ackIDs := make([]string, len(msgs))
		for i, msg := range msgs {
			if !*reportMPS {
				fmt.Printf("Got a message: %s\n", msg.Data)
			}
			ackIDs[i] = msg.AckID
		}
		go ack(ctx, sub, ackIDs...)
	}
}

func pullMessages(ctx context.Context, argv []string) {
	checkArgs(argv, 3)
	sub := argv[1]
	workers, err := strconv.Atoi(argv[2])
	if err != nil {
		log.Fatalf("Atoi failed, %v", err)
	}
	result := make(chan int, 1024)
	for i := 0; i < int(workers); i++ {
		go pullLoop(ctx, sub, result)
	}
	if *reportMPS {
		r := reporter{reportTitle: "Received", result: result}
		r.report()
	} else {
		select {}
	}
}

func publishLoop(ctx context.Context, topic string, workerid int, result chan<- int) {
	var r uint64
	for {
		msgs := make([]*pubsub.Message, *size)
		for i := 0; i < *size; i++ {
			msgs[i] = &pubsub.Message{
				Data: []byte(fmt.Sprintf("Worker: %d, Round: %d, Message: %d", workerid, r, i)),
			}
		}
		_, err := pubsub.Publish(ctx, topic, msgs...)
		if err != nil {
			log.Printf("Publish failed, %v\n", err)
			return
		}
		r++
		if *reportMPS {
			result <- *size
		}
	}
}

func publishMessages(ctx context.Context, argv []string) {
	checkArgs(argv, 3)
	topic := argv[1]
	workers, err := strconv.Atoi(argv[2])
	if err != nil {
		log.Fatalf("Atoi failed, %v", err)
	}
	result := make(chan int, 1024)
	for i := 0; i < int(workers); i++ {
		go publishLoop(ctx, topic, i, result)
	}
	if *reportMPS {
		r := reporter{reportTitle: "Sent", result: result}
		r.report()
	} else {
		select {}
	}
}

// This example demonstrates calling the Cloud Pub/Sub API. As of 22
// Oct 2014, the Cloud Pub/Sub API is only available if you're
// whitelisted. If you're interested in using it, please apply for the
// Limited Preview program at the following form:
// http://goo.gl/Wql9HL
//
// Also, before running this example, be sure to enable Cloud Pub/Sub
// service on your project in Developer Console at:
// https://console.developers.google.com/
//
// Unless you run this sample on Compute Engine instance, please
// create a new service account and download a JSON key file for it at
// the developer console: https://console.developers.google.com/
//
// It has the following subcommands:
//
//  create_topic <name>
//  delete_topic <name>
//  create_subscription <name> <linked_topic>
//  delete_subscription <name>
//  publish <topic> <message>
//  pull_messages <subscription> <numworkers>
//  publish_messages <topic> <numworkers>
//
// You can choose any names for topic and subscription as long as they
// follow the naming rule described at:
// https://cloud.google.com/pubsub/overview#names
//
// You can create/delete topics/subscriptions by self-explanatory
// subcommands.
//
// The "publish" subcommand is for publishing a single message to a
// specified Cloud Pub/Sub topic.
//
// The "pull_messages" subcommand is for continuously pulling messages
// from a specified Cloud Pub/Sub subscription with specified number
// of workers.
//
// The "publish_messages" subcommand is for continuously publishing
// messages to a specified Cloud Pub/Sub topic with specified number
// of workers.
func main() {
	flag.Parse()
	argv := flag.Args()
	checkArgs(argv, 1)
	client, err := newClient(*jsonFile)
	if err != nil {
		log.Fatalf("clientAndId failed, %v", err)
	}
	if *projID == "" {
		usageAndExit("Please specify Project ID.")
	}
	ctx := cloud.NewContext(*projID, client)
	m := map[string]func(ctx context.Context, argv []string){
		"create_topic":        createTopic,
		"delete_topic":        deleteTopic,
		"create_subscription": createSubscription,
		"delete_subscription": deleteSubscription,
		"publish":             publish,
		"pull_messages":       pullMessages,
		"publish_messages":    publishMessages,
	}
	subcommand := argv[0]
	f, ok := m[subcommand]
	if !ok {
		usageAndExit(fmt.Sprintf("Function not found for %s", subcommand))
	}
	f(ctx, argv)
}
