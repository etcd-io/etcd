/*
Copyright 2013 CoreOS Inc.

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

package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"

	"github.com/BurntSushi/toml"
)

//--------------------------------------
// Config
//--------------------------------------

// Config holds the etcd and raft server configurations.
type Config struct {
	ConfigFile string
	Etcd       EtcdConfig
	Raft       RaftConfig
}

// EtcdConfig represents the etcd server configuration.
type EtcdConfig struct {
	AdvertisedUrl    string   `toml:"advertised_url"`
	CAFile           string   `toml:"ca_file"`
	CertFile         string   `toml:"cert_file"`
	CPUProfileFile   string   `toml:"cpu_profile_file"`
	Cors             []string `toml:"cors"`
	CorsWhiteList    map[string]bool
	DataDir          string   `toml:"datadir"`
	KeyFile          string   `toml:"key_file"`
	ListenHost       string   `toml:"listen_host"`
	Machines         []string `toml:"machines"`
	MachinesFile     string   `toml:"machines_file"`
	MaxClusterSize   int      `toml:"max_cluster_size"`
	MaxResultBuffer  int      `toml:"max_result_buffer"`
	MaxRetryAttempts int      `toml:"max_retry_attempts"`
	Name             string   `toml:"name"`
	Snapshot         bool     `toml:"snapshot"`
	TLSConfig        TLSConfig
	TLSInfo          TLSInfo
	Verbose          bool   `toml:"verbose"`
	VeryVerbose      bool   `toml:"very_verbose"`
	WebURL           string `toml:"web_url"`
}

// RaftConfig represents the raft server configuration.
type RaftConfig struct {
	AdvertisedUrl string `toml:"advertised_url"`
	CAFile        string `toml:"ca_file"`
	CertFile      string `toml:"cert_file"`
	KeyFile       string `toml:"key_file"`
	ListenHost    string `toml:"listen_host"`
	TLSConfig     TLSConfig
	TLSInfo       TLSInfo
}

// TLSConfig holds the TLS configuration.
type TLSConfig struct {
	Scheme string
	Server tls.Config
	Client tls.Config
}

// TLSInfo holds the SSL certificates paths.
type TLSInfo struct {
	CAFile   string
	CertFile string
	KeyFile  string
}

// NewConfig return a Config initialized with default values.
func NewConfig() *Config {
	c := new(Config)
	c.Etcd.AdvertisedUrl = "127.0.0.1:4001"
	c.Etcd.CorsWhiteList = make(map[string]bool)
	c.Etcd.DataDir = "."
	c.Etcd.MaxClusterSize = 9
	c.Etcd.MaxResultBuffer = 1024
	c.Etcd.MaxRetryAttempts = 3
	c.Raft.AdvertisedUrl = "127.0.0.1:7001"
	return c
}

// processConfig sets the etcd and raft configuration by processing configuration
// sources starting with the etcd configuration file. Setting are overridden by
// the next configuration source.
func (c *Config) processConfig() {
	c.setConfigFromFile()
	c.setConfigFromEnv()
	c.setConfigFromFlags()
	c.setMachines()
	c.setCorsWhiteList()
	c.setEtcdTLSInfo()
	c.setEtcdTLSConfig()
	c.setRaftTLSInfo()
	c.setRaftTLSConfig()
	c.sanitize()
	c.check()
}

// setConfigFile sets the path to the etcd configuration file.
func (c *Config) setConfigFile(path string) {
	c.ConfigFile = path
}

// setConfigFromFile sets configuration options from a configuration file.
// It logs and exits if there are any errors.
func (c *Config) setConfigFromFile() {
	if c.ConfigFile == "" {
		return
	}
	// Ignore the metadata as we don't need it.
	_, err := toml.DecodeFile(c.ConfigFile, &c)
	if err != nil {
		fatalf("Unable to parse config: %v", err)
	}
}

// setConfigFromEnv sets configuration options from environment variables.
func (c *Config) setConfigFromEnv() {
	envToVar("ETCD_ADVERTISED_URL", c.Etcd.AdvertisedUrl)
	envToVar("ETCD_CA_FILE", c.Etcd.CAFile)
	envToVar("ETCD_CERT_FILE", c.Etcd.CertFile)
	envToVar("ETCD_CPU_PROFILE_FILE", c.Etcd.CPUProfileFile)
	envToVar("ETCD_DATADIR", c.Etcd.DataDir)
	envToVar("ETCD_KEY_FILE", c.Etcd.KeyFile)
	envToVar("ETCD_LISTEN_HOST", c.Etcd.ListenHost)
	envToVar("ETCD_MAX_CLUSTER_SIZE", c.Etcd.MaxClusterSize)
	envToVar("ETCD_MAX_RESULTS_BUFFER", c.Etcd.MaxResultBuffer)
	envToVar("ETCD_MAX_RETRY_ATTEMPTS", c.Etcd.MaxRetryAttempts)
	envToVar("ETCD_NAME", c.Etcd.Name)
	envToVar("ETCD_SNAPSHOT", c.Etcd.Snapshot)
	envToVar("ETCD_VERBOSE", c.Etcd.Verbose)
	envToVar("ETCD_VERY_VERBOSE", c.Etcd.VeryVerbose)
	envToVar("ETCD_WEB_URL", c.Etcd.WebURL)
	envToVar("ETCD_RAFT_ADVERTISED_URL", c.Raft.AdvertisedUrl)
	envToVar("ETCD_RAFT_CA_FILE", c.Raft.CAFile)
	envToVar("ETCD_RAFT_CERT_FILE", c.Raft.CertFile)
	envToVar("ETCD_RAFT_KEY_FILE", c.Raft.KeyFile)
	envToVar("ETCD_RAFT_LISTEN_HOST", c.Raft.ListenHost)
}

// setConfigFromFlags sets configuration options from the command line flags.
func (c *Config) setConfigFromFlags() {
	flag.Visit(c.overrideFromFlags)
}

// overrideFromFlags overrides configuration settings based on values passed
// in through command line flags.
func (c *Config) overrideFromFlags(f *flag.Flag) {
	switch f.Name {
	case "c":
		c.Etcd.AdvertisedUrl = etcdAdvertisedUrl
	case "cl":
		c.Etcd.ListenHost = etcdListenHost
	case "clientCAFile":
		c.Etcd.CAFile = etcdCAFile
	case "clientCert":
		c.Etcd.CertFile = etcdCertFile
	case "clientKey":
		c.Etcd.KeyFile = etcdKeyFile
	case "cpuprofile":
		c.Etcd.CPUProfileFile = etcdCPUProfileFile
	case "d":
		c.Etcd.DataDir = etcdDataDir
	case "n":
		c.Etcd.Name = etcdName
	case "maxsize":
		c.Etcd.MaxClusterSize = etcdMaxClusterSize
	case "m":
		c.Etcd.MaxResultBuffer = etcdMaxResultBuffer
	case "r":
		c.Etcd.MaxRetryAttempts = etcdMaxRetryAttempts
	case "s":
		c.Raft.AdvertisedUrl = raftAdvertisedUrl
	case "sl":
		c.Raft.ListenHost = raftListenHost
	case "serverCAFile":
		c.Raft.CAFile = raftCAFile
	case "serverCert":
		c.Raft.CertFile = raftCertFile
	case "serverKey":
		c.Raft.KeyFile = raftKeyFile
	case "snapshot":
		c.Etcd.Snapshot = etcdSnapshot
	case "w":
		c.Etcd.WebURL = etcdWebURL
	case "v":
		c.Etcd.Verbose = etcdVerbose
	case "vv":
		c.Etcd.VeryVerbose = etcdVeryVerbose
	}
}

// setMachines sets the etcd cluster members.
func (c *Config) setMachines() {
	// First check if a machine list was supplied on the command line.
	// Then look to the environment.
	if machines == "" {
		machines = os.Getenv("ETCD_MACHINES")
	}
	if machinesFile == "" {
		machinesFile = os.Getenv("ETCD_MACHINES_FILE")
	}
	if machinesFile == "" {
		machinesFile = c.Etcd.MachinesFile
	}
	if machines != "" {
		c.Etcd.Machines = splitAndTrimSpace(machines, ",")
	} else if machinesFile != "" {
		b, err := ioutil.ReadFile(machinesFile)
		if err != nil {
			fatalf("Unable to read the given machines file: %s", err)
		}
		c.Etcd.Machines = splitAndTrimSpace(string(b), ",")
	}
}

// setEtcdTLSInfo sets the etcd TLSInfo.
func (c *Config) setEtcdTLSInfo() {
	c.Etcd.TLSInfo = TLSInfo{
		CAFile:   c.Etcd.CAFile,
		CertFile: c.Etcd.CertFile,
		KeyFile:  c.Etcd.KeyFile,
	}
}

// setRaftTLSInfo sets the raft TLSInfo.
func (c *Config) setRaftTLSInfo() {
	c.Raft.TLSInfo = TLSInfo{
		CAFile:   c.Raft.CAFile,
		CertFile: c.Raft.CertFile,
		KeyFile:  c.Raft.KeyFile,
	}
}

// setEtcdTLSConfig sets the etcd TLSConfig.
// It logs and exits if there are any errors.
func (c *Config) setEtcdTLSConfig() {
	var ok bool
	c.Etcd.TLSConfig, ok = tlsConfigFromInfo(config.Etcd.TLSInfo)
	if !ok {
		fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}
}

// setRaftTLSConfig sets the etcd TLSConfig.
// It logs and exits if there are any errors.
func (c *Config) setRaftTLSConfig() {
	var ok bool
	c.Raft.TLSConfig, ok = tlsConfigFromInfo(config.Raft.TLSInfo)
	if !ok {
		fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}
}

// check checks the configuration options.
func (c *Config) check() {
	if config.Etcd.Name == "" {
		fatal("ERROR: server name required. e.g. '-n=server_name'")
	}
}

// sanitize sanitizes the hostname and URL configuration options.
func (c *Config) sanitize() {
	config.Etcd.AdvertisedUrl = sanitizeURL(config.Etcd.AdvertisedUrl, config.Etcd.TLSConfig.Scheme)
	config.Etcd.ListenHost = sanitizeListenHost(config.Etcd.ListenHost, config.Etcd.AdvertisedUrl)
	config.Etcd.WebURL = sanitizeURL(config.Etcd.WebURL, "http")
	config.Raft.AdvertisedUrl = sanitizeURL(config.Raft.AdvertisedUrl, config.Raft.TLSConfig.Scheme)
	config.Raft.ListenHost = sanitizeListenHost(config.Raft.ListenHost, config.Raft.AdvertisedUrl)
}

// setCorsWhiteList sets CorsWhiteList.
// It panics if the cors white list would contains an invalid URL.
func (c *Config) setCorsWhiteList() {
	var corsList []string
	if cors == "" {
		cors = os.Getenv("ETCD_CORS")
	}
	if cors != "" {
		corsList = splitAndTrimSpace(cors, ",")
	} else {
		corsList = c.Etcd.Cors
	}
	if len(corsList) > 0 {
		for _, v := range corsList {
			fmt.Println(v)
			if v != "*" {
				_, err := url.Parse(v)
				if err != nil {
					panic(fmt.Sprintf("bad cors url: %s", err))
				}
			}
			c.Etcd.CorsWhiteList[v] = true
		}
	}
}

func tlsConfigFromInfo(info TLSInfo) (t TLSConfig, ok bool) {
	var keyFile, certFile, CAFile string
	var tlsCert tls.Certificate
	var err error

	t.Scheme = "http"

	keyFile = info.KeyFile
	certFile = info.CertFile
	CAFile = info.CAFile

	// If the user do not specify key file, cert file and
	// CA file, the type will be HTTP
	if keyFile == "" && certFile == "" && CAFile == "" {
		return t, true
	}

	// both the key and cert must be present
	if keyFile == "" || certFile == "" {
		return t, false
	}

	tlsCert, err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		fatal(err)
	}

	t.Scheme = "https"
	t.Server.ClientAuth, t.Server.ClientCAs = newCertPool(CAFile)

	// The client should trust the RootCA that the Server uses since
	// everyone is a peer in the network.
	t.Client.Certificates = []tls.Certificate{tlsCert}
	t.Client.RootCAs = t.Server.ClientCAs

	return t, true
}

// newCertPool creates x509 certPool and corresponding Auth Type.
// If the given CAfile is valid, add the cert into the pool and verify the clients'
// certs against the cert in the pool.
// If the given CAfile is empty, do not verify the clients' cert.
// If the given CAfile is not valid, fatal.
func newCertPool(CAFile string) (tls.ClientAuthType, *x509.CertPool) {
	if CAFile == "" {
		return tls.NoClientCert, nil
	}
	pemByte, err := ioutil.ReadFile(CAFile)
	check(err)

	block, pemByte := pem.Decode(pemByte)

	cert, err := x509.ParseCertificate(block.Bytes)
	check(err)

	certPool := x509.NewCertPool()

	certPool.AddCert(cert)

	return tls.RequireAndVerifyClientCert, certPool
}
