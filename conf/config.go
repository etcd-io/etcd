package conf

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// The default location for the etcd configuration file.
const DefaultSystemConfigPath = "/etc/etcd/etcd.conf"

// Config represents the server configuration.
type Config struct {
	SystemPath string

	Addr             string `env:"ETCD_ADDR"`
	BindAddr         string `env:"ETCD_BIND_ADDR"`
	CAFile           string `env:"ETCD_CA_FILE"`
	CertFile         string `env:"ETCD_CERT_FILE"`
	CPUProfileFile   string
	CorsOrigins      []string `env:"ETCD_CORS"`
	DataDir          string   `env:"ETCD_DATA_DIR"`
	Discovery        string   `env:"ETCD_DISCOVERY"`
	Force            bool
	KeyFile          string   `env:"ETCD_KEY_FILE"`
	HTTPReadTimeout  float64  `env:"ETCD_HTTP_READ_TIMEOUT"`
	HTTPWriteTimeout float64  `env:"ETCD_HTTP_WRITE_TIMEOUT"`
	Peers            []string `env:"ETCD_PEERS"`
	PeersFile        string   `env:"ETCD_PEERS_FILE"`
	MaxResultBuffer  int      `env:"ETCD_MAX_RESULT_BUFFER"`
	MaxRetryAttempts int      `env:"ETCD_MAX_RETRY_ATTEMPTS"`
	RetryInterval    float64  `env:"ETCD_RETRY_INTERVAL"`
	Name             string   `env:"ETCD_NAME"`
	Snapshot         bool     `env:"ETCD_SNAPSHOT"`
	SnapshotCount    int      `env:"ETCD_SNAPSHOTCOUNT"`
	ShowHelp         bool
	ShowVersion      bool
	Verbose          bool `env:"ETCD_VERBOSE"`
	VeryVerbose      bool `env:"ETCD_VERY_VERBOSE"`
	VeryVeryVerbose  bool `env:"ETCD_VERY_VERY_VERBOSE"`
	Peer             struct {
		Addr              string `env:"ETCD_PEER_ADDR"`
		BindAddr          string `env:"ETCD_PEER_BIND_ADDR"`
		CAFile            string `env:"ETCD_PEER_CA_FILE"`
		CertFile          string `env:"ETCD_PEER_CERT_FILE"`
		KeyFile           string `env:"ETCD_PEER_KEY_FILE"`
		HeartbeatInterval int    `env:"ETCD_PEER_HEARTBEAT_INTERVAL"`
		ElectionTimeout   int    `env:"ETCD_PEER_ELECTION_TIMEOUT"`
	}
	strTrace     string `env:"ETCD_TRACE"`
	GraphiteHost string `env:"ETCD_GRAPHITE_HOST"`
	Cluster      struct {
		ActiveSize   int     `env:"ETCD_CLUSTER_ACTIVE_SIZE"`
		RemoveDelay  float64 `env:"ETCD_CLUSTER_REMOVE_DELAY"`
		SyncInterval float64 `env:"ETCD_CLUSTER_SYNC_INTERVAL"`
	}
}

// New returns a Config initialized with default values.
func New() *Config {
	c := new(Config)
	c.SystemPath = DefaultSystemConfigPath
	c.Addr = "127.0.0.1:4001"
	c.HTTPReadTimeout = DefaultReadTimeout
	c.HTTPWriteTimeout = DefaultWriteTimeout
	c.MaxResultBuffer = 1024
	c.MaxRetryAttempts = 3
	c.RetryInterval = 10.0
	c.Snapshot = true
	c.SnapshotCount = 10000
	c.Peer.Addr = "127.0.0.1:7001"
	c.Peer.HeartbeatInterval = DefaultHeartbeatInterval
	c.Peer.ElectionTimeout = DefaultElectionTimeout
	rand.Seed(time.Now().UTC().UnixNano())
	// Make maximum twice as minimum.
	c.RetryInterval = float64(50+rand.Int()%50) * DefaultHeartbeatInterval / 1000
	c.Cluster.ActiveSize = DefaultActiveSize
	c.Cluster.RemoveDelay = DefaultRemoveDelay
	c.Cluster.SyncInterval = DefaultSyncInterval
	return c
}

// Loads the configuration from the system config, command line config,
// environment variables, and finally command line arguments.
func (c *Config) Load(arguments []string) error {
	var path string
	f := flag.NewFlagSet("etcd", -1)
	f.SetOutput(ioutil.Discard)
	f.StringVar(&path, "config", "", "path to config file")
	f.Parse(arguments)

	// Load from the environment variables next.
	if err := c.LoadEnv(); err != nil {
		return err
	}

	// Load from command line flags.
	if err := c.LoadFlags(arguments); err != nil {
		return err
	}

	// Loads peers if a peer file was specified.
	if err := c.LoadPeersFile(); err != nil {
		return err
	}

	return nil
}

// LoadEnv loads the configuration via environment variables.
func (c *Config) LoadEnv() error {
	if err := c.loadEnv(c); err != nil {
		return err
	}
	if err := c.loadEnv(&c.Peer); err != nil {
		return err
	}
	if err := c.loadEnv(&c.Cluster); err != nil {
		return err
	}
	return nil
}

func (c *Config) loadEnv(target interface{}) error {
	value := reflect.Indirect(reflect.ValueOf(target))
	typ := value.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Retrieve environment variable.
		v := strings.TrimSpace(os.Getenv(field.Tag.Get("env")))
		if v == "" {
			continue
		}

		// Set the appropriate type.
		switch field.Type.Kind() {
		case reflect.Bool:
			value.Field(i).SetBool(v != "0" && v != "false")
		case reflect.Int:
			newValue, err := strconv.ParseInt(v, 10, 0)
			if err != nil {
				return fmt.Errorf("Parse error: %s: %s", field.Tag.Get("env"), err)
			}
			value.Field(i).SetInt(newValue)
		case reflect.String:
			value.Field(i).SetString(v)
		case reflect.Slice:
			value.Field(i).Set(reflect.ValueOf(trimSplit(v, ",")))
		case reflect.Float64:
			newValue, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("Parse error: %s: %s", field.Tag.Get("env"), err)
			}
			value.Field(i).SetFloat(newValue)
		}
	}
	return nil
}

// Loads configuration from command line flags.
func (c *Config) LoadFlags(arguments []string) error {
	var peers, cors, path string

	f := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	f.SetOutput(ioutil.Discard)

	f.BoolVar(&c.ShowHelp, "h", false, "")
	f.BoolVar(&c.ShowHelp, "help", false, "")
	f.BoolVar(&c.ShowVersion, "version", false, "")

	f.BoolVar(&c.Force, "f", false, "")
	f.BoolVar(&c.Force, "force", false, "")

	f.BoolVar(&c.Verbose, "v", c.Verbose, "")
	f.BoolVar(&c.VeryVerbose, "vv", c.VeryVerbose, "")
	f.BoolVar(&c.VeryVeryVerbose, "vvv", c.VeryVeryVerbose, "")

	f.StringVar(&peers, "peers", "", "")
	f.StringVar(&c.PeersFile, "peers-file", c.PeersFile, "")

	f.StringVar(&c.Name, "name", c.Name, "")
	f.StringVar(&c.Addr, "addr", c.Addr, "")
	f.StringVar(&c.Discovery, "discovery", c.Discovery, "")
	f.StringVar(&c.BindAddr, "bind-addr", c.BindAddr, "")
	f.StringVar(&c.Peer.Addr, "peer-addr", c.Peer.Addr, "")
	f.StringVar(&c.Peer.BindAddr, "peer-bind-addr", c.Peer.BindAddr, "")

	f.StringVar(&c.CAFile, "ca-file", c.CAFile, "")
	f.StringVar(&c.CertFile, "cert-file", c.CertFile, "")
	f.StringVar(&c.KeyFile, "key-file", c.KeyFile, "")

	f.StringVar(&c.Peer.CAFile, "peer-ca-file", c.Peer.CAFile, "")
	f.StringVar(&c.Peer.CertFile, "peer-cert-file", c.Peer.CertFile, "")
	f.StringVar(&c.Peer.KeyFile, "peer-key-file", c.Peer.KeyFile, "")

	f.Float64Var(&c.HTTPReadTimeout, "http-read-timeout", c.HTTPReadTimeout, "")
	f.Float64Var(&c.HTTPWriteTimeout, "http-write-timeout", c.HTTPReadTimeout, "")

	f.StringVar(&c.DataDir, "data-dir", c.DataDir, "")
	f.IntVar(&c.MaxResultBuffer, "max-result-buffer", c.MaxResultBuffer, "")
	f.IntVar(&c.MaxRetryAttempts, "max-retry-attempts", c.MaxRetryAttempts, "")
	f.Float64Var(&c.RetryInterval, "retry-interval", c.RetryInterval, "")
	f.IntVar(&c.Peer.HeartbeatInterval, "peer-heartbeat-interval", c.Peer.HeartbeatInterval, "")
	f.IntVar(&c.Peer.ElectionTimeout, "peer-election-timeout", c.Peer.ElectionTimeout, "")

	f.StringVar(&cors, "cors", "", "")

	f.BoolVar(&c.Snapshot, "snapshot", c.Snapshot, "")
	f.IntVar(&c.SnapshotCount, "snapshot-count", c.SnapshotCount, "")
	f.StringVar(&c.CPUProfileFile, "cpuprofile", "", "")

	f.StringVar(&c.strTrace, "trace", "", "")
	f.StringVar(&c.GraphiteHost, "graphite-host", "", "")

	f.IntVar(&c.Cluster.ActiveSize, "cluster-active-size", c.Cluster.ActiveSize, "")
	f.Float64Var(&c.Cluster.RemoveDelay, "cluster-remove-delay", c.Cluster.RemoveDelay, "")
	f.Float64Var(&c.Cluster.SyncInterval, "cluster-sync-interval", c.Cluster.SyncInterval, "")

	// BEGIN IGNORED FLAGS
	f.StringVar(&path, "config", "", "")
	// BEGIN IGNORED FLAGS

	if err := f.Parse(arguments); err != nil {
		return err
	}

	// Convert some parameters to lists.
	if peers != "" {
		c.Peers = trimSplit(peers, ",")
	}
	if cors != "" {
		c.CorsOrigins = trimSplit(cors, ",")
	}

	return nil
}

// LoadPeersFile loads the peers listed in the peers file.
func (c *Config) LoadPeersFile() error {
	if c.PeersFile == "" {
		return nil
	}

	b, err := ioutil.ReadFile(c.PeersFile)
	if err != nil {
		return fmt.Errorf("Peers file error: %s", err)
	}
	c.Peers = trimSplit(string(b), ",")

	return nil
}

// DataDirFromName sets the data dir from a machine name and issue a warning
// that etcd is guessing.
func (c *Config) DataDirFromName() {
	c.DataDir = c.Name + ".etcd"
	log.Printf("Using the directory %s as the etcd curation directory because a directory was not specified. ", c.DataDir)

	return
}

// NameFromHostname sets the machine name from the hostname. This is to help
// people get started without thinking up a name.
func (c *Config) NameFromHostname() {
	host, err := os.Hostname()
	if err != nil && host == "" {
		log.Fatal("Node name required and hostname not set. e.g. '-name=name'")
	}
	c.Name = host
}

// Reset removes all server configuration files.
func (c *Config) Reset() error {
	if err := os.RemoveAll(filepath.Join(c.DataDir, "log")); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(c.DataDir, "conf")); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(c.DataDir, "snapshot")); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(c.DataDir, "standby_info")); err != nil {
		return err
	}

	return nil
}

// Sanitize cleans the input fields.
func (c *Config) Sanitize() error {
	var err error
	var url *url.URL

	// Sanitize the URLs first.
	if c.Addr, url, err = sanitizeURL(c.Addr, c.EtcdTLSInfo().Scheme()); err != nil {
		return fmt.Errorf("Advertised URL: %s", err)
	}
	if c.BindAddr, err = sanitizeBindAddr(c.BindAddr, url); err != nil {
		return fmt.Errorf("Listen Host: %s", err)
	}
	if c.Peer.Addr, url, err = sanitizeURL(c.Peer.Addr, c.PeerTLSInfo().Scheme()); err != nil {
		return fmt.Errorf("Peer Advertised URL: %s", err)
	}
	if c.Peer.BindAddr, err = sanitizeBindAddr(c.Peer.BindAddr, url); err != nil {
		return fmt.Errorf("Peer Listen Host: %s", err)
	}

	// Only guess the machine name if there is no data dir specified
	// because the info file should have our name
	if c.Name == "" && c.DataDir == "" {
		c.NameFromHostname()
	}

	if c.DataDir == "" && c.Name != "" && !c.ShowVersion && !c.ShowHelp {
		c.DataDirFromName()
	}

	return nil
}

// EtcdTLSInfo retrieves a TLSInfo object for the etcd server
func (c *Config) EtcdTLSInfo() *TLSInfo {
	return &TLSInfo{
		CAFile:   c.CAFile,
		CertFile: c.CertFile,
		KeyFile:  c.KeyFile,
	}
}

// PeerRaftInfo retrieves a TLSInfo object for the peer server.
func (c *Config) PeerTLSInfo() *TLSInfo {
	return &TLSInfo{
		CAFile:   c.Peer.CAFile,
		CertFile: c.Peer.CertFile,
		KeyFile:  c.Peer.KeyFile,
	}
}

// MetricsBucketName generates the name that should be used for a
// corresponding MetricsBucket object
func (c *Config) MetricsBucketName() string {
	return fmt.Sprintf("etcd.%s", c.Name)
}

// Trace determines if any trace-level information should be emitted
func (c *Config) Trace() bool {
	return c.strTrace == "*"
}

func (c *Config) ClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		ActiveSize:   c.Cluster.ActiveSize,
		RemoveDelay:  c.Cluster.RemoveDelay,
		SyncInterval: c.Cluster.SyncInterval,
	}
}

// sanitizeURL will cleanup a host string in the format hostname[:port] and
// attach a schema.
func sanitizeURL(host string, defaultScheme string) (string, *url.URL, error) {
	// Blank URLs are fine input, just return it
	if len(host) == 0 {
		return host, &url.URL{}, nil
	}

	p, err := url.Parse(host)
	if err != nil {
		return "", nil, err
	}

	// Make sure the host is in Host:Port format
	_, _, err = net.SplitHostPort(host)
	if err != nil {
		return "", nil, err
	}

	p = &url.URL{Host: host, Scheme: defaultScheme}
	return p.String(), p, nil
}

// sanitizeBindAddr cleans up the BindAddr parameter and appends a port
// if necessary based on the advertised port.
func sanitizeBindAddr(bindAddr string, aurl *url.URL) (string, error) {
	// If it is a valid host:port simply return with no further checks.
	bhost, bport, err := net.SplitHostPort(bindAddr)
	if err == nil && bhost != "" {
		return bindAddr, nil
	}

	// SplitHostPort makes the host optional, but we don't want that.
	if bhost == "" && bport != "" {
		return "", fmt.Errorf("IP required can't use a port only")
	}

	// bindAddr doesn't have a port if we reach here so take the port from the
	// advertised URL.
	_, aport, err := net.SplitHostPort(aurl.Host)
	if err != nil {
		return "", err
	}

	return net.JoinHostPort(bindAddr, aport), nil
}

// trimSplit slices s into all substrings separated by sep and returns a
// slice of the substrings between the separator with all leading and trailing
// white space removed, as defined by Unicode.
func trimSplit(s, sep string) []string {
	trimmed := strings.Split(s, sep)
	for i := range trimmed {
		trimmed[i] = strings.TrimSpace(trimmed[i])
	}
	return trimmed
}
