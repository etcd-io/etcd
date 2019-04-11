package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	handling "go.etcd.io/etcd/discoveryserver/http"

	"github.com/coreos/go-systemd/activation"
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

	viper.BindPFlag("etcd", pflag.Lookup("etcd"))
	viper.BindPFlag("host", pflag.Lookup("host"))
	viper.BindPFlag("addr", pflag.Lookup("addr"))

	pflag.Parse()
}

func main() {
	log.SetFlags(0)
	etcdHost := mustHostOnlyURL(viper.GetString("etcd"))
	discHost := mustHostOnlyURL(viper.GetString("host"))
	webAddr := viper.GetString("addr")

	handling.Setup(context.Background(), etcdHost, discHost)

	log.Printf("discovery server started with etcd %q and host %q", etcdHost, discHost)
	log.Printf("discovery serving on %s", webAddr)
	err := http.ListenAndServe(webAddr, nil)
	if err != nil {
		panic(err)
	}

	listeners, err := activation.Listeners()
	if err != nil {
		panic(err)
	}

	if len(listeners) != 1 {
		panic("Unexpected number of socket activation fds")
	}

	http.Serve(listeners[0], nil)
}
