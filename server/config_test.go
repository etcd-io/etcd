package server

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
)


// Ensures that a configuration can be deserialized from TOML.
func TestConfigTOML(t *testing.T) {
	content := `
		advertised_url = "127.0.0.1:4002"
		ca_file = "/tmp/file.ca"
		cert_file = "/tmp/file.cert"
		cors = ["*"]
		cpu_profile_file = "XXX"
		datadir = "/tmp/data"
		key_file = "/tmp/file.key"
		listen_host = "127.0.0.1:4003"
		machines = ["coreos.com:4001", "coreos.com:4002"]
		machines_file = "/tmp/machines"
		max_cluster_size = 10
		max_result_buffer = 512
		max_retry_attempts = 5
		name = "test-name"
		snapshot = true
		verbose = true
		very_verbose = true
		web_url = "/web"

		[peer]
		advertised_url = "127.0.0.1:7002"
		ca_file = "/tmp/peer/file.ca"
		cert_file = "/tmp/peer/file.cert"
		key_file = "/tmp/peer/file.key"
		listen_host = "127.0.0.1:7003"
	`
	c := NewConfig()
	_, err := toml.Decode(content, &c)
	assert.Nil(t, err, "")
	assert.Equal(t, c.AdvertisedUrl, "127.0.0.1:4002", "")
	assert.Equal(t, c.CAFile, "/tmp/file.ca", "")
	assert.Equal(t, c.CertFile, "/tmp/file.cert", "")
	assert.Equal(t, c.Cors, []string{"*"}, "")
	assert.Equal(t, c.DataDir, "/tmp/data", "")
	assert.Equal(t, c.KeyFile, "/tmp/file.key", "")
	assert.Equal(t, c.ListenHost, "127.0.0.1:4003", "")
	assert.Equal(t, c.Machines, []string{"coreos.com:4001", "coreos.com:4002"}, "")
	assert.Equal(t, c.MachinesFile, "/tmp/machines", "")
	assert.Equal(t, c.MaxClusterSize, 10, "")
	assert.Equal(t, c.MaxResultBuffer, 512, "")
	assert.Equal(t, c.MaxRetryAttempts, 5, "")
	assert.Equal(t, c.Name, "test-name", "")
	assert.Equal(t, c.Snapshot, true, "")
	assert.Equal(t, c.Verbose, true, "")
	assert.Equal(t, c.VeryVerbose, true, "")
	assert.Equal(t, c.WebURL, "/web", "")
	assert.Equal(t, c.Peer.AdvertisedUrl, "127.0.0.1:7002", "")
	assert.Equal(t, c.Peer.CAFile, "/tmp/peer/file.ca", "")
	assert.Equal(t, c.Peer.CertFile, "/tmp/peer/file.cert", "")
	assert.Equal(t, c.Peer.KeyFile, "/tmp/peer/file.key", "")
	assert.Equal(t, c.Peer.ListenHost, "127.0.0.1:7003", "")
}

// Ensures that a configuration can be retrieved from environment variables.
func TestConfigEnv(t *testing.T) {
	os.Setenv("ETCD_CA_FILE", "/tmp/file.ca")
	os.Setenv("ETCD_CERT_FILE", "/tmp/file.cert")
	os.Setenv("ETCD_CPU_PROFILE_FILE", "XXX")
	os.Setenv("ETCD_CORS", "localhost:4001,localhost:4002")
	os.Setenv("ETCD_DATADIR", "/tmp/data")
	os.Setenv("ETCD_KEY_FILE", "/tmp/file.key")
	os.Setenv("ETCD_LISTEN_HOST", "127.0.0.1:4003")
	os.Setenv("ETCD_MACHINES", "coreos.com:4001,coreos.com:4002")
	os.Setenv("ETCD_MACHINES_FILE", "/tmp/machines")
	os.Setenv("ETCD_MAX_CLUSTER_SIZE", "10")
	os.Setenv("ETCD_MAX_RESULT_BUFFER", "512")
	os.Setenv("ETCD_MAX_RETRY_ATTEMPTS", "5")
	os.Setenv("ETCD_NAME", "test-name")
	os.Setenv("ETCD_SNAPSHOT", "true")
	os.Setenv("ETCD_VERBOSE", "1")
	os.Setenv("ETCD_VERY_VERBOSE", "yes")
	os.Setenv("ETCD_WEB_URL", "/web")
	os.Setenv("ETCD_PEER_ADVERTISED_URL", "127.0.0.1:7002")
	os.Setenv("ETCD_PEER_CA_FILE", "/tmp/peer/file.ca")
	os.Setenv("ETCD_PEER_CERT_FILE", "/tmp/peer/file.cert")
	os.Setenv("ETCD_PEER_KEY_FILE", "/tmp/peer/file.key")
	os.Setenv("ETCD_PEER_LISTEN_HOST", "127.0.0.1:7003")
	
	c := NewConfig()
	c.LoadEnv()
	assert.Equal(t, c.CAFile, "/tmp/file.ca", "")
	assert.Equal(t, c.CertFile, "/tmp/file.cert", "")
	assert.Equal(t, c.Cors, []string{"localhost:4001", "localhost:4002"}, "")
	assert.Equal(t, c.DataDir, "/tmp/data", "")
	assert.Equal(t, c.KeyFile, "/tmp/file.key", "")
	assert.Equal(t, c.ListenHost, "127.0.0.1:4003", "")
	assert.Equal(t, c.Machines, []string{"coreos.com:4001", "coreos.com:4002"}, "")
	assert.Equal(t, c.MachinesFile, "/tmp/machines", "")
	assert.Equal(t, c.MaxClusterSize, 10, "")
	assert.Equal(t, c.MaxResultBuffer, 512, "")
	assert.Equal(t, c.MaxRetryAttempts, 5, "")
	assert.Equal(t, c.Name, "test-name", "")
	assert.Equal(t, c.Snapshot, true, "")
	assert.Equal(t, c.Verbose, true, "")
	assert.Equal(t, c.VeryVerbose, true, "")
	assert.Equal(t, c.WebURL, "/web", "")
	assert.Equal(t, c.Peer.AdvertisedUrl, "127.0.0.1:7002", "")
	assert.Equal(t, c.Peer.CAFile, "/tmp/peer/file.ca", "")
	assert.Equal(t, c.Peer.CertFile, "/tmp/peer/file.cert", "")
	assert.Equal(t, c.Peer.KeyFile, "/tmp/peer/file.key", "")
	assert.Equal(t, c.Peer.ListenHost, "127.0.0.1:7003", "")
}

// Ensures that a the advertised url can be parsed from the environment.
func TestConfigAdvertisedUrlEnv(t *testing.T) {
	withEnv("ETCD_ADVERTISED_URL", "127.0.0.1:4002", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.AdvertisedUrl, "127.0.0.1:4002", "")
	})
}

// Ensures that a the advertised flag can be parsed.
func TestConfigAdvertisedUrlFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-c", "127.0.0.1:4002"}), "")
	assert.Equal(t, c.AdvertisedUrl, "127.0.0.1:4002", "")
}

// Ensures that a the CA file can be parsed from the environment.
func TestConfigCAFileEnv(t *testing.T) {
	withEnv("ETCD_CA_FILE", "/tmp/file.ca", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.CAFile, "/tmp/file.ca", "")
	})
}

// Ensures that a the CA file flag can be parsed.
func TestConfigCAFileFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-clientCAFile", "/tmp/file.ca"}), "")
	assert.Equal(t, c.CAFile, "/tmp/file.ca", "")
}

// Ensures that a the CA file can be parsed from the environment.
func TestConfigCertFileEnv(t *testing.T) {
	withEnv("ETCD_CERT_FILE", "/tmp/file.cert", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.CertFile, "/tmp/file.cert", "")
	})
}

// Ensures that a the Cert file flag can be parsed.
func TestConfigCertFileFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-clientCert", "/tmp/file.cert"}), "")
	assert.Equal(t, c.CertFile, "/tmp/file.cert", "")
}

// Ensures that a the Key file can be parsed from the environment.
func TestConfigKeyFileEnv(t *testing.T) {
	withEnv("ETCD_KEY_FILE", "/tmp/file.key", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.KeyFile, "/tmp/file.key", "")
	})
}

// Ensures that a the Key file flag can be parsed.
func TestConfigKeyFileFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-clientKey", "/tmp/file.key"}), "")
	assert.Equal(t, c.KeyFile, "/tmp/file.key", "")
}

// Ensures that a the Listen Host can be parsed from the environment.
func TestConfigListenHostEnv(t *testing.T) {
	withEnv("ETCD_LISTEN_HOST", "127.0.0.1:4003", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.ListenHost, "127.0.0.1:4003", "")
	})
}

// Ensures that a the Listen Host file flag can be parsed.
func TestConfigListenHostFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-cl", "127.0.0.1:4003"}), "")
	assert.Equal(t, c.ListenHost, "127.0.0.1:4003", "")
}

// Ensures that the Machines can be parsed from the environment.
func TestConfigMachinesEnv(t *testing.T) {
	withEnv("ETCD_MACHINES", "coreos.com:4001,coreos.com:4002", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Machines, []string{"coreos.com:4001", "coreos.com:4002"}, "")
	})
}

// Ensures that a the Machines flag can be parsed.
func TestConfigMachinesFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-C", "coreos.com:4001,coreos.com:4002"}), "")
	assert.Equal(t, c.Machines, []string{"coreos.com:4001", "coreos.com:4002"}, "")
}

// Ensures that the Machines File can be parsed from the environment.
func TestConfigMachinesFileEnv(t *testing.T) {
	withEnv("ETCD_MACHINES_FILE", "/tmp/machines", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.MachinesFile, "/tmp/machines", "")
	})
}

// Ensures that a the Machines File flag can be parsed.
func TestConfigMachinesFileFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-CF", "/tmp/machines"}), "")
	assert.Equal(t, c.MachinesFile, "/tmp/machines", "")
}

// Ensures that the Max Cluster Size can be parsed from the environment.
func TestConfigMaxClusterSizeEnv(t *testing.T) {
	withEnv("ETCD_MAX_CLUSTER_SIZE", "5", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.MaxClusterSize, 5, "")
	})
}

// Ensures that a the Max Cluster Size flag can be parsed.
func TestConfigMaxClusterSizeFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-maxsize", "5"}), "")
	assert.Equal(t, c.MaxClusterSize, 5, "")
}

// Ensures that the Max Result Buffer can be parsed from the environment.
func TestConfigMaxResultBufferEnv(t *testing.T) {
	withEnv("ETCD_MAX_RESULT_BUFFER", "512", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.MaxResultBuffer, 512, "")
	})
}

// Ensures that a the Max Result Buffer flag can be parsed.
func TestConfigMaxResultBufferFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-m", "512"}), "")
	assert.Equal(t, c.MaxResultBuffer, 512, "")
}

// Ensures that the Max Retry Attempts can be parsed from the environment.
func TestConfigMaxRetryAttemptsEnv(t *testing.T) {
	withEnv("ETCD_MAX_RETRY_ATTEMPTS", "10", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.MaxRetryAttempts, 10, "")
	})
}

// Ensures that a the Max Retry Attempts flag can be parsed.
func TestConfigMaxRetryAttemptsFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-r", "10"}), "")
	assert.Equal(t, c.MaxRetryAttempts, 10, "")
}

// Ensures that the Name can be parsed from the environment.
func TestConfigNameEnv(t *testing.T) {
	withEnv("ETCD_NAME", "test-name", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Name, "test-name", "")
	})
}

// Ensures that a the Name flag can be parsed.
func TestConfigNameFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-n", "test-name"}), "")
	assert.Equal(t, c.Name, "test-name", "")
}

// Ensures that Snapshot can be parsed from the environment.
func TestConfigSnapshotEnv(t *testing.T) {
	withEnv("ETCD_SNAPSHOT", "1", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Snapshot, true, "")
	})
}

// Ensures that a the Snapshot flag can be parsed.
func TestConfigSnapshotFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-snapshot"}), "")
	assert.Equal(t, c.Snapshot, true, "")
}

// Ensures that Verbose can be parsed from the environment.
func TestConfigVerboseEnv(t *testing.T) {
	withEnv("ETCD_VERBOSE", "true", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Verbose, true, "")
	})
}

// Ensures that a the Verbose flag can be parsed.
func TestConfigVerboseFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-v"}), "")
	assert.Equal(t, c.Verbose, true, "")
}

// Ensures that Very Verbose can be parsed from the environment.
func TestConfigVeryVerboseEnv(t *testing.T) {
	withEnv("ETCD_VERY_VERBOSE", "true", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.VeryVerbose, true, "")
	})
}

// Ensures that a the Very Verbose flag can be parsed.
func TestConfigVeryVerboseFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-vv"}), "")
	assert.Equal(t, c.VeryVerbose, true, "")
}

// Ensures that Web URL can be parsed from the environment.
func TestConfigWebURLEnv(t *testing.T) {
	withEnv("ETCD_WEB_URL", "/web", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.WebURL, "/web", "")
	})
}

// Ensures that a the Web URL flag can be parsed.
func TestConfigWebURLFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-w", "/web"}), "")
	assert.Equal(t, c.WebURL, "/web", "")
}

// Ensures that the Peer Advertised URL can be parsed from the environment.
func TestConfigPeerAdvertisedUrlEnv(t *testing.T) {
	withEnv("ETCD_PEER_ADVERTISED_URL", "localhost:7002", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Peer.AdvertisedUrl, "localhost:7002", "")
	})
}

// Ensures that a the Peer Advertised URL flag can be parsed.
func TestConfigPeerAdvertisedUrlFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-s", "localhost:7002"}), "")
	assert.Equal(t, c.Peer.AdvertisedUrl, "localhost:7002", "")
}

// Ensures that the Peer CA File can be parsed from the environment.
func TestConfigPeerCAFileEnv(t *testing.T) {
	withEnv("ETCD_PEER_CA_FILE", "/tmp/peer/file.ca", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Peer.CAFile, "/tmp/peer/file.ca", "")
	})
}

// Ensures that a the Peer CA file flag can be parsed.
func TestConfigPeerCAFileFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-serverCAFile", "/tmp/peer/file.ca"}), "")
	assert.Equal(t, c.Peer.CAFile, "/tmp/peer/file.ca", "")
}

// Ensures that the Peer Cert File can be parsed from the environment.
func TestConfigPeerCertFileEnv(t *testing.T) {
	withEnv("ETCD_PEER_CERT_FILE", "/tmp/peer/file.cert", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Peer.CertFile, "/tmp/peer/file.cert", "")
	})
}

// Ensures that a the Cert file flag can be parsed.
func TestConfigPeerCertFileFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-serverCert", "/tmp/peer/file.cert"}), "")
	assert.Equal(t, c.Peer.CertFile, "/tmp/peer/file.cert", "")
}

// Ensures that the Peer Key File can be parsed from the environment.
func TestConfigPeerKeyFileEnv(t *testing.T) {
	withEnv("ETCD_PEER_KEY_FILE", "/tmp/peer/file.key", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Peer.KeyFile, "/tmp/peer/file.key", "")
	})
}

// Ensures that a the Peer Key file flag can be parsed.
func TestConfigPeerKeyFileFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-serverKey", "/tmp/peer/file.key"}), "")
	assert.Equal(t, c.Peer.KeyFile, "/tmp/peer/file.key", "")
}

// Ensures that the Peer Listen Host can be parsed from the environment.
func TestConfigPeerListenHostEnv(t *testing.T) {
	withEnv("ETCD_PEER_LISTEN_HOST", "localhost:7004", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Peer.ListenHost, "localhost:7004", "")
	})
}

// Ensures that a the Peer Listen Host file flag can be parsed.
func TestConfigPeerListenHostFlag(t *testing.T) {
	c := NewConfig()
	assert.Nil(t, c.LoadFlags([]string{"-sl", "127.0.0.1:4003"}), "")
	assert.Equal(t, c.Peer.ListenHost, "127.0.0.1:4003", "")
}


// Ensures that a system config field is overridden by a custom config field.
func TestConfigCustomConfigOverrideSystemConfig(t *testing.T) {
	system := `advertised_url = "127.0.0.1:5000"`
	custom := `advertised_url = "127.0.0.1:6000"`
	withTempFile(system, func(p1 string) {
		withTempFile(custom, func(p2 string) {
			c := NewConfig()
			c.SystemPath = p1
			assert.Nil(t, c.Load([]string{"-config", p2}), "")
			assert.Equal(t, c.AdvertisedUrl, "http://127.0.0.1:6000", "")
		})
	})
}

// Ensures that a custom config field is overridden by an environment variable.
func TestConfigEnvVarOverrideCustomConfig(t *testing.T) {
	os.Setenv("ETCD_PEER_ADVERTISED_URL", "127.0.0.1:8000")
	defer os.Setenv("ETCD_PEER_ADVERTISED_URL", "")

	custom := `[peer]`+"\n"+`advertised_url = "127.0.0.1:9000"`
	withTempFile(custom, func(path string) {
		c := NewConfig()
		c.SystemPath = ""
		assert.Nil(t, c.Load([]string{"-config", path}), "")
		assert.Equal(t, c.Peer.AdvertisedUrl, "http://127.0.0.1:8000", "")
	})
}

// Ensures that an environment variable field is overridden by a command line argument.
func TestConfigCLIArgsOverrideEnvVar(t *testing.T) {
	os.Setenv("ETCD_ADVERTISED_URL", "127.0.0.1:1000")
	defer os.Setenv("ETCD_ADVERTISED_URL", "")

	c := NewConfig()
	c.SystemPath = ""
	assert.Nil(t, c.Load([]string{"-c", "127.0.0.1:2000"}), "")
	assert.Equal(t, c.AdvertisedUrl, "http://127.0.0.1:2000", "")
}


//--------------------------------------
// Helpers
//--------------------------------------

// Sets up the environment with a given environment variable set.
func withEnv(key, value string, f func(c *Config)) {
	os.Setenv(key, value)
	defer os.Setenv(key, "")
	c := NewConfig()
	f(c)
}

// Creates a temp file and calls a function with the context.
func withTempFile(content string, fn func(string)) {
	f, _ := ioutil.TempFile("", "")
	f.WriteString(content)
	f.Close()
	defer os.Remove(f.Name())
	fn(f.Name())
}
