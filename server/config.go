package server

// Config represents the server configuration.
type Config struct {
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
	Verbose          bool     `toml:"verbose"`
	VeryVerbose      bool     `toml:"very_verbose"`
	WebURL           string   `toml:"web_url"`

	Peer struct {
		AdvertisedUrl string `toml:"advertised_url"`
		CAFile        string `toml:"ca_file"`
		CertFile      string `toml:"cert_file"`
		KeyFile       string `toml:"key_file"`
		ListenHost    string `toml:"listen_host"`
	}
}

// NewConfig returns a Config initialized with default values.
func NewConfig() *Config {
	c := new(Config)
	c.AdvertisedUrl = "127.0.0.1:4001"
	c.CorsWhiteList = make(map[string]bool)
	c.DataDir = "."
	c.MaxClusterSize = 9
	c.MaxResultBuffer = 1024
	c.MaxRetryAttempts = 3
	c.Peer.AdvertisedUrl = "127.0.0.1:7001"
	return c
}
