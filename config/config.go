package config

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/third_party/github.com/BurntSushi/toml"

	"github.com/coreos/etcd/log"
	ustrings "github.com/coreos/etcd/pkg/strings"
	"github.com/coreos/etcd/server"
)

// The default location for the etcd configuration file.
const DefaultSystemConfigPath = "/etc/etcd/etcd.conf"

// A lookup of deprecated flags to their new flag name.
var newFlagNameLookup = map[string]string{
	"C":                      "peers",
	"CF":                     "peers-file",
	"n":                      "name",
	"c":                      "addr",
	"cl":                     "bind-addr",
	"s":                      "peer-addr",
	"sl":                     "peer-bind-addr",
	"d":                      "data-dir",
	"m":                      "max-result-buffer",
	"r":                      "max-retry-attempts",
	"maxsize":                "cluster-active-size",
	"clientCAFile":           "ca-file",
	"clientCert":             "cert-file",
	"clientKey":              "key-file",
	"serverCAFile":           "peer-ca-file",
	"serverCert":             "peer-cert-file",
	"serverKey":              "peer-key-file",
	"snapshotCount":          "snapshot-count",
	"peer-heartbeat-timeout": "peer-heartbeat-interval",
	"max-cluster-size":       "cluster-active-size",
}

// Config represents the server configuration.
type Config struct {
	SystemPath string

	Addr             string `toml:"addr" env:"ETCD_ADDR"`
	BindAddr         string `toml:"bind_addr" env:"ETCD_BIND_ADDR"`
	CAFile           string `toml:"ca_file" env:"ETCD_CA_FILE"`
	CertFile         string `toml:"cert_file" env:"ETCD_CERT_FILE"`
	CPUProfileFile   string
	CorsOrigins      []string `toml:"cors" env:"ETCD_CORS"`
	DataDir          string   `toml:"data_dir" env:"ETCD_DATA_DIR"`
	Discovery        string   `toml:"discovery" env:"ETCD_DISCOVERY"`
	Force            bool
	KeyFile          string   `toml:"key_file" env:"ETCD_KEY_FILE"`
	HTTPReadTimeout  float64  `toml:"http_read_timeout" env:"ETCD_HTTP_READ_TIMEOUT"`
	HTTPWriteTimeout float64  `toml:"http_write_timeout" env:"ETCD_HTTP_WRITE_TIMEOUT"`
	Peers            []string `toml:"peers" env:"ETCD_PEERS"`
	PeersFile        string   `toml:"peers_file" env:"ETCD_PEERS_FILE"`
	MaxResultBuffer  int      `toml:"max_result_buffer" env:"ETCD_MAX_RESULT_BUFFER"`
	MaxRetryAttempts int      `toml:"max_retry_attempts" env:"ETCD_MAX_RETRY_ATTEMPTS"`
	RetryInterval    float64  `toml:"retry_interval" env:"ETCD_RETRY_INTERVAL"`
	Name             string   `toml:"name" env:"ETCD_NAME"`
	Snapshot         bool     `toml:"snapshot" env:"ETCD_SNAPSHOT"`
	SnapshotCount    int      `toml:"snapshot_count" env:"ETCD_SNAPSHOTCOUNT"`
	ShowHelp         bool
	ShowVersion      bool
	Verbose          bool `toml:"verbose" env:"ETCD_VERBOSE"`
	VeryVerbose      bool `toml:"very_verbose" env:"ETCD_VERY_VERBOSE"`
	VeryVeryVerbose  bool `toml:"very_very_verbose" env:"ETCD_VERY_VERY_VERBOSE"`
	Peer             struct {
		Addr              string `toml:"addr" env:"ETCD_PEER_ADDR"`
		BindAddr          string `toml:"bind_addr" env:"ETCD_PEER_BIND_ADDR"`
		CAFile            string `toml:"ca_file" env:"ETCD_PEER_CA_FILE"`
		CertFile          string `toml:"cert_file" env:"ETCD_PEER_CERT_FILE"`
		KeyFile           string `toml:"key_file" env:"ETCD_PEER_KEY_FILE"`
		HeartbeatInterval int    `toml:"heartbeat_interval" env:"ETCD_PEER_HEARTBEAT_INTERVAL"`
		ElectionTimeout   int    `toml:"election_timeout" env:"ETCD_PEER_ELECTION_TIMEOUT"`
	}
	strTrace     string `toml:"trace" env:"ETCD_TRACE"`
	GraphiteHost string `toml:"graphite_host" env:"ETCD_GRAPHITE_HOST"`
	Cluster      struct {
		ActiveSize   int     `toml:"active_size" env:"ETCD_CLUSTER_ACTIVE_SIZE"`
		RemoveDelay  float64 `toml:"remove_delay" env:"ETCD_CLUSTER_REMOVE_DELAY"`
		SyncInterval float64 `toml:"sync_interval" env:"ETCD_CLUSTER_SYNC_INTERVAL"`
	}
}

// New returns a Config initialized with default values.
func New() *Config {
	c := new(Config)
	c.SystemPath = DefaultSystemConfigPath
	c.Addr = "127.0.0.1:4001"
	c.HTTPReadTimeout = server.DefaultReadTimeout
	c.HTTPWriteTimeout = server.DefaultWriteTimeout
	c.MaxResultBuffer = 1024
	c.MaxRetryAttempts = 3
	c.RetryInterval = 10.0
	c.Snapshot = true
	c.SnapshotCount = 10000
	c.Peer.Addr = "127.0.0.1:7001"
	c.Peer.HeartbeatInterval = defaultHeartbeatInterval
	c.Peer.ElectionTimeout = defaultElectionTimeout
	rand.Seed(time.Now().UTC().UnixNano())
	// Make maximum twice as minimum.
	c.RetryInterval = float64(50+rand.Int()%50) * defaultHeartbeatInterval / 1000
	c.Cluster.ActiveSize = server.DefaultActiveSize
	c.Cluster.RemoveDelay = server.DefaultRemoveDelay
	c.Cluster.SyncInterval = server.DefaultSyncInterval
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

	// Load from system file.
	if err := c.LoadSystemFile(); err != nil {
		return err
	}

	// Load from config file specified in arguments.
	if path != "" {
		if err := c.LoadFile(path); err != nil {
			return err
		}
	}

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

// Loads from the system etcd configuration file if it exists.
func (c *Config) LoadSystemFile() error {
	if _, err := os.Stat(c.SystemPath); os.IsNotExist(err) {
		return nil
	}
	return c.LoadFile(c.SystemPath)
}

// Loads configuration from a file.
func (c *Config) LoadFile(path string) error {
	_, err := toml.DecodeFile(path, &c)
	return err
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
			value.Field(i).Set(reflect.ValueOf(ustrings.TrimSplit(v, ",")))
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

	// BEGIN DEPRECATED FLAGS
	f.StringVar(&peers, "C", "", "(deprecated)")
	f.StringVar(&c.PeersFile, "CF", c.PeersFile, "(deprecated)")
	f.StringVar(&c.Name, "n", c.Name, "(deprecated)")
	f.StringVar(&c.Addr, "c", c.Addr, "(deprecated)")
	f.StringVar(&c.BindAddr, "cl", c.BindAddr, "(deprecated)")
	f.StringVar(&c.Peer.Addr, "s", c.Peer.Addr, "(deprecated)")
	f.StringVar(&c.Peer.BindAddr, "sl", c.Peer.BindAddr, "(deprecated)")
	f.StringVar(&c.Peer.CAFile, "serverCAFile", c.Peer.CAFile, "(deprecated)")
	f.StringVar(&c.Peer.CertFile, "serverCert", c.Peer.CertFile, "(deprecated)")
	f.StringVar(&c.Peer.KeyFile, "serverKey", c.Peer.KeyFile, "(deprecated)")
	f.StringVar(&c.CAFile, "clientCAFile", c.CAFile, "(deprecated)")
	f.StringVar(&c.CertFile, "clientCert", c.CertFile, "(deprecated)")
	f.StringVar(&c.KeyFile, "clientKey", c.KeyFile, "(deprecated)")
	f.StringVar(&c.DataDir, "d", c.DataDir, "(deprecated)")
	f.IntVar(&c.MaxResultBuffer, "m", c.MaxResultBuffer, "(deprecated)")
	f.IntVar(&c.MaxRetryAttempts, "r", c.MaxRetryAttempts, "(deprecated)")
	f.IntVar(&c.SnapshotCount, "snapshotCount", c.SnapshotCount, "(deprecated)")
	f.IntVar(&c.Peer.HeartbeatInterval, "peer-heartbeat-timeout", c.Peer.HeartbeatInterval, "(deprecated)")
	f.IntVar(&c.Cluster.ActiveSize, "max-cluster-size", c.Cluster.ActiveSize, "(deprecated)")
	f.IntVar(&c.Cluster.ActiveSize, "maxsize", c.Cluster.ActiveSize, "(deprecated)")
	// END DEPRECATED FLAGS

	if err := f.Parse(arguments); err != nil {
		return err
	}

	// Print deprecation warnings on STDERR.
	f.Visit(func(f *flag.Flag) {
		if len(newFlagNameLookup[f.Name]) > 0 {
			fmt.Fprintf(os.Stderr, "[deprecated] use -%s, not -%s\n", newFlagNameLookup[f.Name], f.Name)
		}
	})

	// Convert some parameters to lists.
	if peers != "" {
		c.Peers = ustrings.TrimSplit(peers, ",")
	}
	if cors != "" {
		c.CorsOrigins = ustrings.TrimSplit(cors, ",")
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
	c.Peers = ustrings.TrimSplit(string(b), ",")

	return nil
}

// DataDirFromName sets the data dir from a machine name and issue a warning
// that etcd is guessing.
func (c *Config) DataDirFromName() {
	c.DataDir = c.Name + ".etcd"
	log.Warnf("Using the directory %s as the etcd curation directory because a directory was not specified. ", c.DataDir)

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
func (c *Config) EtcdTLSInfo() *server.TLSInfo {
	return &server.TLSInfo{
		CAFile:   c.CAFile,
		CertFile: c.CertFile,
		KeyFile:  c.KeyFile,
	}
}

// PeerRaftInfo retrieves a TLSInfo object for the peer server.
func (c *Config) PeerTLSInfo() *server.TLSInfo {
	return &server.TLSInfo{
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

func (c *Config) ClusterConfig() *server.ClusterConfig {
	return &server.ClusterConfig{
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
