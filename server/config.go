package server

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// The default location for the etcd configuration file.
const DefaultSystemConfigPath = "/etc/etcd/etcd.conf"

// Config represents the server configuration.
type Config struct {
	SystemPath string

	AdvertisedUrl    string   `toml:"advertised_url" env:"ETCD_ADVERTISED_URL"`
	CAFile           string   `toml:"ca_file" env:"ETCD_CA_FILE"`
	CertFile         string   `toml:"cert_file" env:"ETCD_CERT_FILE"`
	Cors             []string `toml:"cors" env:"ETCD_CORS"`
	DataDir          string   `toml:"datadir" env:"ETCD_DATADIR"`
	KeyFile          string   `toml:"key_file" env:"ETCD_KEY_FILE"`
	ListenHost       string   `toml:"listen_host" env:"ETCD_LISTEN_HOST"`
	Machines         []string `toml:"machines" env:"ETCD_MACHINES"`
	MachinesFile     string   `toml:"machines_file" env:"ETCD_MACHINES_FILE"`
	MaxClusterSize   int      `toml:"max_cluster_size" env:"ETCD_MAX_CLUSTER_SIZE"`
	MaxResultBuffer  int      `toml:"max_result_buffer" env:"ETCD_MAX_RESULT_BUFFER"`
	MaxRetryAttempts int      `toml:"max_retry_attempts" env:"ETCD_MAX_RETRY_ATTEMPTS"`
	Name             string   `toml:"name" env:"ETCD_NAME"`
	Snapshot         bool     `toml:"snapshot" env:"ETCD_SNAPSHOT"`
	SnapCount        int      `toml:"snap_count" env:"ETCD_SNAPCOUNT"`
	Verbose          bool     `toml:"verbose" env:"ETCD_VERBOSE"`
	VeryVerbose      bool     `toml:"very_verbose" env:"ETCD_VERY_VERBOSE"`
	WebURL           string   `toml:"web_url" env:"ETCD_WEB_URL"`

	Peer struct {
		AdvertisedUrl string `toml:"advertised_url" env:"ETCD_PEER_ADVERTISED_URL"`
		CAFile        string `toml:"ca_file" env:"ETCD_PEER_CA_FILE"`
		CertFile      string `toml:"cert_file" env:"ETCD_PEER_CERT_FILE"`
		KeyFile       string `toml:"key_file" env:"ETCD_PEER_KEY_FILE"`
		ListenHost    string `toml:"listen_host" env:"ETCD_PEER_LISTEN_HOST"`
	}
}

// NewConfig returns a Config initialized with default values.
func NewConfig() *Config {
	c := new(Config)
	c.SystemPath = DefaultSystemConfigPath
	c.AdvertisedUrl = "127.0.0.1:4001"
	c.AdvertisedUrl = "127.0.0.1:4001"
	c.DataDir = "."
	c.MaxClusterSize = 9
	c.MaxResultBuffer = 1024
	c.MaxRetryAttempts = 3
	c.Peer.AdvertisedUrl = "127.0.0.1:7001"
	c.SnapCount = 10000
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

	// Loads machines if a machine file was specified.
	if err := c.LoadMachineFile(); err != nil {
		return err
	}

	// Sanitize all the input fields.
	if err := c.Sanitize(); err != nil {
		return fmt.Errorf("sanitize:", err)
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
			value.Field(i).Set(reflect.ValueOf(trimsplit(v, ",")))
		}
	}
	return nil
}

// Loads configuration from command line flags.
func (c *Config) LoadFlags(arguments []string) error {
	var machines, cors string
	var force bool

	f := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	f.BoolVar(&force, "f", false, "force new node configuration if existing is found (WARNING: data loss!)")

	f.BoolVar(&c.Verbose, "v", c.Verbose, "verbose logging")
	f.BoolVar(&c.VeryVerbose, "vv", c.Verbose, "very verbose logging")

	f.StringVar(&machines, "C", "", "the ip address and port of a existing machines in the cluster, sepearate by comma")
	f.StringVar(&c.MachinesFile, "CF", c.MachinesFile, "the file contains a list of existing machines in the cluster, seperate by comma")

	f.StringVar(&c.Name, "n", c.Name, "the node name (required)")
	f.StringVar(&c.AdvertisedUrl, "c", c.AdvertisedUrl, "the advertised public hostname:port for etcd client communication")
	f.StringVar(&c.Peer.AdvertisedUrl, "s", c.Peer.AdvertisedUrl, "the advertised public hostname:port for raft server communication")
	f.StringVar(&c.ListenHost, "cl", c.ListenHost, "the listening hostname for etcd client communication (defaults to advertised ip)")
	f.StringVar(&c.Peer.ListenHost, "sl", c.Peer.ListenHost, "the listening hostname for raft server communication (defaults to advertised ip)")
	f.StringVar(&c.WebURL, "w", c.WebURL, "the hostname:port of web interface")

	f.StringVar(&c.Peer.CAFile, "serverCAFile", c.Peer.CAFile, "the path of the CAFile")
	f.StringVar(&c.Peer.CertFile, "serverCert", c.Peer.CertFile, "the cert file of the server")
	f.StringVar(&c.Peer.KeyFile, "serverKey", c.Peer.KeyFile, "the key file of the server")

	f.StringVar(&c.CAFile, "clientCAFile", c.CAFile, "the path of the client CAFile")
	f.StringVar(&c.CertFile, "clientCert", c.CertFile, "the cert file of the client")
	f.StringVar(&c.KeyFile, "clientKey", c.KeyFile, "the key file of the client")

	f.StringVar(&c.DataDir, "d", c.DataDir, "the directory to store log and snapshot")
	f.IntVar(&c.MaxResultBuffer, "m", c.MaxResultBuffer, "the max size of result buffer")
	f.IntVar(&c.MaxRetryAttempts, "r", c.MaxRetryAttempts, "the max retry attempts when trying to join a cluster")
	f.IntVar(&c.MaxClusterSize, "maxsize", c.MaxClusterSize, "the max size of the cluster")
	f.StringVar(&cors, "cors", "", "whitelist origins for cross-origin resource sharing (e.g. '*' or 'http://localhost:8001,etc')")

	f.BoolVar(&c.Snapshot, "snapshot", c.Snapshot, "open or close snapshot")
	f.IntVar(&c.SnapCount, "snapCount", c.SnapCount, "save the in memory logs and states to a snapshot file after snapCount transactions")

	// These flags are ignored since they were already parsed.
	var path string
	f.StringVar(&path, "config", "", "path to config file")

	f.Parse(arguments)

	// Convert some parameters to lists.
	if machines != "" {
		c.Machines = trimsplit(machines, ",")
	}
	if cors != "" {
		c.Cors = trimsplit(cors, ",")
	}

	// Force remove server configuration if specified.
	if force {
		c.Reset()
	}

	return nil
}

// LoadMachineFile loads the machines listed in the machine file.
func (c *Config) LoadMachineFile() error {
	if c.MachinesFile == "" {
		return nil
	}

	b, err := ioutil.ReadFile(c.MachinesFile)
	if err != nil {
		return fmt.Errorf("Machines file error: %s", err)
	}
	c.Machines = trimsplit(string(b), ",")

	return nil
}

// Reset removes all server configuration files.
func (c *Config) Reset() error {
	if err := os.RemoveAll(filepath.Join(c.DataDir, "info")); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(c.DataDir, "log")); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(c.DataDir, "conf")); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(c.DataDir, "snapshot")); err != nil {
		return err
	}

	return nil
}

// Reads the info file from the file system or initializes it based on the config.
func (c *Config) Info() (*Info, error) {
	info := &Info{}
	path := filepath.Join(c.DataDir, "info")

	// Open info file and read it out.
	f, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if f != nil {
		defer f.Close()
		if err := json.NewDecoder(f).Decode(&info); err != nil {
			return nil, err
		}
		return info, nil
	}

	// If the file doesn't exist then initialize it.
	info.Name = strings.TrimSpace(c.Name)
	info.EtcdURL = c.AdvertisedUrl
	info.EtcdListenHost = c.ListenHost
	info.RaftURL = c.Peer.AdvertisedUrl
	info.RaftListenHost = c.Peer.ListenHost
	info.WebURL = c.WebURL
	info.EtcdTLS = c.TLSInfo()
	info.RaftTLS = c.PeerTLSInfo()

	// Write to file.
	f, err = os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(info); err != nil {
		return nil, err
	}

	return info, nil
}

// Sanitize cleans the input fields.
func (c *Config) Sanitize() error {
	tlsConfig, err := c.TLSConfig()
	if err != nil {
		return err
	}

	peerTlsConfig, err := c.PeerTLSConfig()
	if err != nil {
		return err
	}

	// Sanitize the URLs first.
	if c.AdvertisedUrl, err = sanitizeURL(c.AdvertisedUrl, tlsConfig.Scheme); err != nil {
		return fmt.Errorf("Advertised URL: %s", err)
	}
	if c.ListenHost, err = sanitizeListenHost(c.ListenHost, c.AdvertisedUrl); err != nil {
		return fmt.Errorf("Listen Host: %s", err)
	}
	if c.WebURL, err = sanitizeURL(c.WebURL, "http"); err != nil {
		return fmt.Errorf("Web URL: %s", err)
	}
	if c.Peer.AdvertisedUrl, err = sanitizeURL(c.Peer.AdvertisedUrl, peerTlsConfig.Scheme); err != nil {
		return fmt.Errorf("Peer Advertised URL: %s", err)
	}
	if c.Peer.ListenHost, err = sanitizeListenHost(c.Peer.ListenHost, c.Peer.AdvertisedUrl); err != nil {
		return fmt.Errorf("Peer Listen Host: %s", err)
	}

	return nil
}

// TLSInfo retrieves a TLSInfo object for the client server.
func (c *Config) TLSInfo() TLSInfo {
	return TLSInfo{
		CAFile:   c.CAFile,
		CertFile: c.CertFile,
		KeyFile:  c.KeyFile,
	}
}

// ClientTLSConfig generates the TLS configuration for the client server.
func (c *Config) TLSConfig() (TLSConfig, error) {
	return c.TLSInfo().Config()
}

// PeerTLSInfo retrieves a TLSInfo object for the peer server.
func (c *Config) PeerTLSInfo() TLSInfo {
	return TLSInfo{
		CAFile:   c.Peer.CAFile,
		CertFile: c.Peer.CertFile,
		KeyFile:  c.Peer.KeyFile,
	}
}

// PeerTLSConfig generates the TLS configuration for the peer server.
func (c *Config) PeerTLSConfig() (TLSConfig, error) {
	return c.PeerTLSInfo().Config()
}

// sanitizeURL will cleanup a host string in the format hostname:port and
// attach a schema.
func sanitizeURL(host string, defaultScheme string) (string, error) {
	// Blank URLs are fine input, just return it
	if len(host) == 0 {
		return host, nil
	}

	p, err := url.Parse(host)
	if err != nil {
		return "", err
	}

	// Make sure the host is in Host:Port format
	_, _, err = net.SplitHostPort(host)
	if err != nil {
		return "", err
	}

	p = &url.URL{Host: host, Scheme: defaultScheme}
	return p.String(), nil
}

// sanitizeListenHost cleans up the ListenHost parameter and appends a port
// if necessary based on the advertised port.
func sanitizeListenHost(listen string, advertised string) (string, error) {
	aurl, err := url.Parse(advertised)
	if err != nil {
		return "", err
	}

	ahost, aport, err := net.SplitHostPort(aurl.Host)
	if err != nil {
		return "", err
	}

	// If the listen host isn't set use the advertised host
	if listen == "" {
		listen = ahost
	}

	return net.JoinHostPort(listen, aport), nil
}
