// Copyright 2019 The etcd Authors
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

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	handling "go.etcd.io/etcd/discoveryserver/http"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func fail(err string) {
	log.Print(err)
	pflag.PrintDefaults()
	os.Exit(2) // default go flag error code
}

func mustHostOnlyURL(givenUrl string) string {
	u, err := url.Parse(givenUrl)

	if err != nil {
		fail(fmt.Sprintf("Invalid url given: %v", err))
	}

	if len(u.Path) != 0 && u.Path != "/" {
		fail(fmt.Sprintf("Expected url without path (%v)", u.Path))
	}

	if u.RawQuery != "" {
		fail(fmt.Sprintf("Expected url without query (?%v)", u.RawQuery))
	}

	if u.Fragment != "" {
		fail(fmt.Sprintf("Expected url without fragment (%v)", u.Fragment))
	}

	if u.Host == "" {
		fail(fmt.Sprint("Expected hostname (none given)"))
	}

	return u.Scheme + "://" + u.Host
}

func init() {
	viper.SetEnvPrefix("disc")
	viper.AutomaticEnv()

	pflag.StringP("etcd", "e", "http://127.0.0.1:2379", "etcd endpoint location")
	pflag.StringP("host", "h", "https://discovery.etcd.io", "discovery url prefix")
	pflag.StringP("addr", "a", ":8087", "web service address")
	pflag.StringP("minage", "m", "168h", "min age of a token for garbage collection")
	pflag.StringP("gcfreq", "g", "5m", "garbage collection frequency")

	viper.BindPFlag("etcd", pflag.Lookup("etcd"))
	viper.BindPFlag("host", pflag.Lookup("host"))
	viper.BindPFlag("addr", pflag.Lookup("addr"))
	viper.BindPFlag("minage", pflag.Lookup("minage"))
	viper.BindPFlag("gcfreq", pflag.Lookup("gcfreq"))

	pflag.Parse()
}

func main() {
	log.SetFlags(0)
	etcdHost := mustHostOnlyURL(viper.GetString("etcd"))
	discHost := mustHostOnlyURL(viper.GetString("host"))
	minAge, err := time.ParseDuration(viper.GetString("minage"))
	if err != nil {
		panic(err)
	}
	gcFreq, err := time.ParseDuration(viper.GetString("gcfreq"))
	if err != nil {
		panic(err)
	}
	webAddr := viper.GetString("addr")

	state := handling.Setup(context.Background(), etcdHost, discHost)

	go func() {
		c := time.Tick(gcFreq)
		for range c {
			log.Printf("garbage collection running")
			state.GarbageCollect(minAge, 10*minAge)
		}
	}()

	log.Printf("discovery server started with etcd %q and host %q", etcdHost, discHost)
	log.Printf("discovery serving on %s", webAddr)
	err = http.ListenAndServe(webAddr, nil)
	if err != nil {
		panic(err)
	}
}
