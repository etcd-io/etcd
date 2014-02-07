package config

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/coreos/etcd/third_party/github.com/BurntSushi/toml"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensures that a configuration can be deserialized from TOML.
func TestConfigTOML(t *testing.T) {
	content := `
		addr = "127.0.0.1:4002"
		ca_file = "/tmp/file.ca"
		cert_file = "/tmp/file.cert"
		cors = ["*"]
		cpu_profile_file = "XXX"
		data_dir = "/tmp/data"
		discovery = "http://example.com/foobar"
		key_file = "/tmp/file.key"
		bind_addr = "127.0.0.1:4003"
		peers = ["coreos.com:4001", "coreos.com:4002"]
		peers_file = "/tmp/peers"
		max_cluster_size = 10
		max_result_buffer = 512
		max_retry_attempts = 5
		name = "test-name"
		snapshot = true
		verbose = true
		very_verbose = true

		[peer]
		addr = "127.0.0.1:7002"
		ca_file = "/tmp/peer/file.ca"
		cert_file = "/tmp/peer/file.cert"
		key_file = "/tmp/peer/file.key"
		bind_addr = "127.0.0.1:7003"
	`
	c := New()
	_, err := toml.Decode(content, &c)
	assert.Nil(t, err, "")
	assert.Equal(t, c.Addr, "127.0.0.1:4002", "")
	assert.Equal(t, c.CAFile, "/tmp/file.ca", "")
	assert.Equal(t, c.CertFile, "/tmp/file.cert", "")
	assert.Equal(t, c.CorsOrigins, []string{"*"}, "")
	assert.Equal(t, c.DataDir, "/tmp/data", "")
	assert.Equal(t, c.Discovery, "http://example.com/foobar", "")
	assert.Equal(t, c.KeyFile, "/tmp/file.key", "")
	assert.Equal(t, c.BindAddr, "127.0.0.1:4003", "")
	assert.Equal(t, c.Peers, []string{"coreos.com:4001", "coreos.com:4002"}, "")
	assert.Equal(t, c.PeersFile, "/tmp/peers", "")
	assert.Equal(t, c.MaxClusterSize, 10, "")
	assert.Equal(t, c.MaxResultBuffer, 512, "")
	assert.Equal(t, c.MaxRetryAttempts, 5, "")
	assert.Equal(t, c.Name, "test-name", "")
	assert.Equal(t, c.Snapshot, true, "")
	assert.Equal(t, c.Verbose, true, "")
	assert.Equal(t, c.VeryVerbose, true, "")
	assert.Equal(t, c.Peer.Addr, "127.0.0.1:7002", "")
	assert.Equal(t, c.Peer.CAFile, "/tmp/peer/file.ca", "")
	assert.Equal(t, c.Peer.CertFile, "/tmp/peer/file.cert", "")
	assert.Equal(t, c.Peer.KeyFile, "/tmp/peer/file.key", "")
	assert.Equal(t, c.Peer.BindAddr, "127.0.0.1:7003", "")
}

// Ensures that a configuration can be retrieved from environment variables.
func TestConfigEnv(t *testing.T) {
	os.Setenv("ETCD_CA_FILE", "/tmp/file.ca")
	os.Setenv("ETCD_CERT_FILE", "/tmp/file.cert")
	os.Setenv("ETCD_CPU_PROFILE_FILE", "XXX")
	os.Setenv("ETCD_CORS", "localhost:4001,localhost:4002")
	os.Setenv("ETCD_DATA_DIR", "/tmp/data")
	os.Setenv("ETCD_DISCOVERY", "http://example.com/foobar")
	os.Setenv("ETCD_KEY_FILE", "/tmp/file.key")
	os.Setenv("ETCD_BIND_ADDR", "127.0.0.1:4003")
	os.Setenv("ETCD_PEERS", "coreos.com:4001,coreos.com:4002")
	os.Setenv("ETCD_PEERS_FILE", "/tmp/peers")
	os.Setenv("ETCD_MAX_CLUSTER_SIZE", "10")
	os.Setenv("ETCD_MAX_RESULT_BUFFER", "512")
	os.Setenv("ETCD_MAX_RETRY_ATTEMPTS", "5")
	os.Setenv("ETCD_NAME", "test-name")
	os.Setenv("ETCD_SNAPSHOT", "true")
	os.Setenv("ETCD_VERBOSE", "1")
	os.Setenv("ETCD_VERY_VERBOSE", "yes")
	os.Setenv("ETCD_PEER_ADDR", "127.0.0.1:7002")
	os.Setenv("ETCD_PEER_CA_FILE", "/tmp/peer/file.ca")
	os.Setenv("ETCD_PEER_CERT_FILE", "/tmp/peer/file.cert")
	os.Setenv("ETCD_PEER_KEY_FILE", "/tmp/peer/file.key")
	os.Setenv("ETCD_PEER_BIND_ADDR", "127.0.0.1:7003")

	c := New()
	c.LoadEnv()
	assert.Equal(t, c.CAFile, "/tmp/file.ca", "")
	assert.Equal(t, c.CertFile, "/tmp/file.cert", "")
	assert.Equal(t, c.CorsOrigins, []string{"localhost:4001", "localhost:4002"}, "")
	assert.Equal(t, c.DataDir, "/tmp/data", "")
	assert.Equal(t, c.Discovery, "http://example.com/foobar", "")
	assert.Equal(t, c.KeyFile, "/tmp/file.key", "")
	assert.Equal(t, c.BindAddr, "127.0.0.1:4003", "")
	assert.Equal(t, c.Peers, []string{"coreos.com:4001", "coreos.com:4002"}, "")
	assert.Equal(t, c.PeersFile, "/tmp/peers", "")
	assert.Equal(t, c.MaxClusterSize, 10, "")
	assert.Equal(t, c.MaxResultBuffer, 512, "")
	assert.Equal(t, c.MaxRetryAttempts, 5, "")
	assert.Equal(t, c.Name, "test-name", "")
	assert.Equal(t, c.Snapshot, true, "")
	assert.Equal(t, c.Verbose, true, "")
	assert.Equal(t, c.VeryVerbose, true, "")
	assert.Equal(t, c.Peer.Addr, "127.0.0.1:7002", "")
	assert.Equal(t, c.Peer.CAFile, "/tmp/peer/file.ca", "")
	assert.Equal(t, c.Peer.CertFile, "/tmp/peer/file.cert", "")
	assert.Equal(t, c.Peer.KeyFile, "/tmp/peer/file.key", "")
	assert.Equal(t, c.Peer.BindAddr, "127.0.0.1:7003", "")

	// Clear this as it will mess up other tests
	os.Setenv("ETCD_DISCOVERY", "")
}

// Ensures that the "help" flag can be parsed.
func TestConfigHelpFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-help"}), "")
	assert.True(t, c.ShowHelp)
}

// Ensures that the abbreviated "help" flag can be parsed.
func TestConfigAbbreviatedHelpFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-h"}), "")
	assert.True(t, c.ShowHelp)
}

// Ensures that the "version" flag can be parsed.
func TestConfigVersionFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-version"}), "")
	assert.True(t, c.ShowVersion)
}

// Ensures that the "force config" flag can be parsed.
func TestConfigForceFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-force"}), "")
	assert.True(t, c.Force)
}

// Ensures that the abbreviated "force config" flag can be parsed.
func TestConfigAbbreviatedForceFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-f"}), "")
	assert.True(t, c.Force)
}

// Ensures that a the advertised url can be parsed from the environment.
func TestConfigAddrEnv(t *testing.T) {
	withEnv("ETCD_ADDR", "127.0.0.1:4002", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Addr, "127.0.0.1:4002", "")
	})
}

// Ensures that a the advertised flag can be parsed.
func TestConfigAddrFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-addr", "127.0.0.1:4002"}), "")
	assert.Equal(t, c.Addr, "127.0.0.1:4002", "")
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-ca-file", "/tmp/file.ca"}), "")
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-cert-file", "/tmp/file.cert"}), "")
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-key-file", "/tmp/file.key"}), "")
	assert.Equal(t, c.KeyFile, "/tmp/file.key", "")
}

// Ensures that a the Listen Host can be parsed from the environment.
func TestConfigBindAddrEnv(t *testing.T) {
	withEnv("ETCD_BIND_ADDR", "127.0.0.1:4003", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.BindAddr, "127.0.0.1:4003", "")
	})
}

// Ensures that a the Listen Host file flag can be parsed.
func TestConfigBindAddrFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-bind-addr", "127.0.0.1:4003"}), "")
	assert.Equal(t, c.BindAddr, "127.0.0.1:4003", "")
}

// Ensures that a the Listen Host port overrides the advertised port
func TestConfigBindAddrOverride(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-addr", "127.0.0.1:4009", "-bind-addr", "127.0.0.1:4010"}), "")
	assert.Nil(t, c.Sanitize())
	assert.Equal(t, c.BindAddr, "127.0.0.1:4010", "")
}

// Ensures that a the Listen Host inherits its port from the advertised addr
func TestConfigBindAddrInheritPort(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-addr", "127.0.0.1:4009", "-bind-addr", "127.0.0.1"}), "")
	assert.Nil(t, c.Sanitize())
	assert.Equal(t, c.BindAddr, "127.0.0.1:4009", "")
}

// Ensures that a port only argument errors out
func TestConfigBindAddrErrorOnNoHost(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-addr", "127.0.0.1:4009", "-bind-addr", ":4010"}), "")
	assert.Error(t, c.Sanitize())
}

// Ensures that the peers can be parsed from the environment.
func TestConfigPeersEnv(t *testing.T) {
	withEnv("ETCD_PEERS", "coreos.com:4001,coreos.com:4002", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Peers, []string{"coreos.com:4001", "coreos.com:4002"}, "")
	})
}

// Ensures that a the Peers flag can be parsed.
func TestConfigPeersFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-peers", "coreos.com:4001,coreos.com:4002"}), "")
	assert.Equal(t, c.Peers, []string{"coreos.com:4001", "coreos.com:4002"}, "")
}

// Ensures that the Peers File can be parsed from the environment.
func TestConfigPeersFileEnv(t *testing.T) {
	withEnv("ETCD_PEERS_FILE", "/tmp/peers", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.PeersFile, "/tmp/peers", "")
	})
}

// Ensures that a the Peers File flag can be parsed.
func TestConfigPeersFileFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-peers-file", "/tmp/peers"}), "")
	assert.Equal(t, c.PeersFile, "/tmp/peers", "")
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-max-cluster-size", "5"}), "")
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-max-result-buffer", "512"}), "")
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-max-retry-attempts", "10"}), "")
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-name", "test-name"}), "")
	assert.Equal(t, c.Name, "test-name", "")
}

// Ensures that a Name gets guessed if not specified
func TestConfigNameGuess(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{}), "")
	assert.Nil(t, c.Sanitize())
	name, _ := os.Hostname()
	assert.Equal(t, c.Name, name, "")
}

// Ensures that a DataDir gets guessed if not specified
func TestConfigDataDirGuess(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{}), "")
	assert.Nil(t, c.Sanitize())
	name, _ := os.Hostname()
	assert.Equal(t, c.DataDir, name+".etcd", "")
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
	c := New()
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
	c := New()
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-vv"}), "")
	assert.Equal(t, c.VeryVerbose, true, "")
}

// Ensures that the Peer Advertised URL can be parsed from the environment.
func TestConfigPeerAddrEnv(t *testing.T) {
	withEnv("ETCD_PEER_ADDR", "localhost:7002", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Peer.Addr, "localhost:7002", "")
	})
}

// Ensures that a the Peer Advertised URL flag can be parsed.
func TestConfigPeerAddrFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-peer-addr", "localhost:7002"}), "")
	assert.Equal(t, c.Peer.Addr, "localhost:7002", "")
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-peer-ca-file", "/tmp/peer/file.ca"}), "")
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-peer-cert-file", "/tmp/peer/file.cert"}), "")
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
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-peer-key-file", "/tmp/peer/file.key"}), "")
	assert.Equal(t, c.Peer.KeyFile, "/tmp/peer/file.key", "")
}

// Ensures that the Peer Listen Host can be parsed from the environment.
func TestConfigPeerBindAddrEnv(t *testing.T) {
	withEnv("ETCD_PEER_BIND_ADDR", "localhost:7004", func(c *Config) {
		assert.Nil(t, c.LoadEnv(), "")
		assert.Equal(t, c.Peer.BindAddr, "localhost:7004", "")
	})
}

// Ensures that a bad flag returns an error.
func TestConfigBadFlag(t *testing.T) {
	c := New()
	err := c.LoadFlags([]string{"-no-such-flag"})
	assert.Error(t, err)
	assert.Equal(t, err.Error(), `flag provided but not defined: -no-such-flag`)
}

// Ensures that a the Peer Listen Host file flag can be parsed.
func TestConfigPeerBindAddrFlag(t *testing.T) {
	c := New()
	assert.Nil(t, c.LoadFlags([]string{"-peer-bind-addr", "127.0.0.1:4003"}), "")
	assert.Equal(t, c.Peer.BindAddr, "127.0.0.1:4003", "")
}

// Ensures that a system config field is overridden by a custom config field.
func TestConfigCustomConfigOverrideSystemConfig(t *testing.T) {
	system := `addr = "127.0.0.1:5000"`
	custom := `addr = "127.0.0.1:6000"`
	withTempFile(system, func(p1 string) {
		withTempFile(custom, func(p2 string) {
			c := New()
			c.SystemPath = p1
			assert.Nil(t, c.Load([]string{"-config", p2}), "")
			assert.Equal(t, c.Addr, "http://127.0.0.1:6000", "")
		})
	})
}

// Ensures that a custom config field is overridden by an environment variable.
func TestConfigEnvVarOverrideCustomConfig(t *testing.T) {
	os.Setenv("ETCD_PEER_ADDR", "127.0.0.1:8000")
	defer os.Setenv("ETCD_PEER_ADDR", "")

	custom := `[peer]` + "\n" + `advertised_url = "127.0.0.1:9000"`
	withTempFile(custom, func(path string) {
		c := New()
		c.SystemPath = ""
		assert.Nil(t, c.Load([]string{"-config", path}), "")
		assert.Equal(t, c.Peer.Addr, "http://127.0.0.1:8000", "")
	})
}

// Ensures that an environment variable field is overridden by a command line argument.
func TestConfigCLIArgsOverrideEnvVar(t *testing.T) {
	os.Setenv("ETCD_ADDR", "127.0.0.1:1000")
	defer os.Setenv("ETCD_ADDR", "")

	c := New()
	c.SystemPath = ""
	assert.Nil(t, c.Load([]string{"-addr", "127.0.0.1:2000"}), "")
	assert.Equal(t, c.Addr, "http://127.0.0.1:2000", "")
}

//--------------------------------------
// DEPRECATED (v1)
//--------------------------------------

func TestConfigDeprecatedAddrFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-c", "127.0.0.1:4002"})
		assert.NoError(t, err)
		assert.Equal(t, c.Addr, "127.0.0.1:4002")
	})
	assert.Equal(t, stderr, "[deprecated] use -addr, not -c\n")
}

func TestConfigDeprecatedBindAddrFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-cl", "127.0.0.1:4003"})
		assert.NoError(t, err)
		assert.Equal(t, c.BindAddr, "127.0.0.1:4003", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -bind-addr, not -cl\n", "")
}

func TestConfigDeprecatedCAFileFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-clientCAFile", "/tmp/file.ca"})
		assert.NoError(t, err)
		assert.Equal(t, c.CAFile, "/tmp/file.ca", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -ca-file, not -clientCAFile\n", "")
}

func TestConfigDeprecatedCertFileFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-clientCert", "/tmp/file.cert"})
		assert.NoError(t, err)
		assert.Equal(t, c.CertFile, "/tmp/file.cert", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -cert-file, not -clientCert\n", "")
}

func TestConfigDeprecatedKeyFileFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-clientKey", "/tmp/file.key"})
		assert.NoError(t, err)
		assert.Equal(t, c.KeyFile, "/tmp/file.key", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -key-file, not -clientKey\n", "")
}

func TestConfigDeprecatedPeersFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-C", "coreos.com:4001,coreos.com:4002"})
		assert.NoError(t, err)
		assert.Equal(t, c.Peers, []string{"coreos.com:4001", "coreos.com:4002"}, "")
	})
	assert.Equal(t, stderr, "[deprecated] use -peers, not -C\n", "")
}

func TestConfigDeprecatedPeersFileFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-CF", "/tmp/machines"})
		assert.NoError(t, err)
		assert.Equal(t, c.PeersFile, "/tmp/machines", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -peers-file, not -CF\n", "")
}

func TestConfigDeprecatedMaxClusterSizeFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-maxsize", "5"})
		assert.NoError(t, err)
		assert.Equal(t, c.MaxClusterSize, 5, "")
	})
	assert.Equal(t, stderr, "[deprecated] use -max-cluster-size, not -maxsize\n", "")
}

func TestConfigDeprecatedMaxResultBufferFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-m", "512"})
		assert.NoError(t, err)
		assert.Equal(t, c.MaxResultBuffer, 512, "")
	})
	assert.Equal(t, stderr, "[deprecated] use -max-result-buffer, not -m\n", "")
}

func TestConfigDeprecatedMaxRetryAttemptsFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-r", "10"})
		assert.NoError(t, err)
		assert.Equal(t, c.MaxRetryAttempts, 10, "")
	})
	assert.Equal(t, stderr, "[deprecated] use -max-retry-attempts, not -r\n", "")
}

func TestConfigDeprecatedNameFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-n", "test-name"})
		assert.NoError(t, err)
		assert.Equal(t, c.Name, "test-name", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -name, not -n\n", "")
}

func TestConfigDeprecatedPeerAddrFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-s", "localhost:7002"})
		assert.NoError(t, err)
		assert.Equal(t, c.Peer.Addr, "localhost:7002", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -peer-addr, not -s\n", "")
}

func TestConfigDeprecatedPeerBindAddrFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-sl", "127.0.0.1:4003"})
		assert.NoError(t, err)
		assert.Equal(t, c.Peer.BindAddr, "127.0.0.1:4003", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -peer-bind-addr, not -sl\n", "")
}

func TestConfigDeprecatedPeerCAFileFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-serverCAFile", "/tmp/peer/file.ca"})
		assert.NoError(t, err)
		assert.Equal(t, c.Peer.CAFile, "/tmp/peer/file.ca", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -peer-ca-file, not -serverCAFile\n", "")
}

func TestConfigDeprecatedPeerCertFileFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-serverCert", "/tmp/peer/file.cert"})
		assert.NoError(t, err)
		assert.Equal(t, c.Peer.CertFile, "/tmp/peer/file.cert", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -peer-cert-file, not -serverCert\n", "")
}

func TestConfigDeprecatedPeerKeyFileFlag(t *testing.T) {
	_, stderr := capture(func() {
		c := New()
		err := c.LoadFlags([]string{"-serverKey", "/tmp/peer/file.key"})
		assert.NoError(t, err)
		assert.Equal(t, c.Peer.KeyFile, "/tmp/peer/file.key", "")
	})
	assert.Equal(t, stderr, "[deprecated] use -peer-key-file, not -serverKey\n", "")
}

//--------------------------------------
// Helpers
//--------------------------------------

// Sets up the environment with a given environment variable set.
func withEnv(key, value string, f func(c *Config)) {
	os.Setenv(key, value)
	defer os.Setenv(key, "")
	c := New()
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

// Captures STDOUT & STDERR and returns the output as strings.
func capture(fn func()) (string, string) {
	// Create temp files.
	tmpout, _ := ioutil.TempFile("", "")
	defer os.Remove(tmpout.Name())
	tmperr, _ := ioutil.TempFile("", "")
	defer os.Remove(tmperr.Name())

	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = tmpout, tmperr

	// Execute function argument and then reassign stdout/stderr.
	fn()
	os.Stdout, os.Stderr = stdout, stderr

	// Close temp files and read them.
	tmpout.Close()
	bout, _ := ioutil.ReadFile(tmpout.Name())
	tmperr.Close()
	berr, _ := ioutil.ReadFile(tmperr.Name())

	return string(bout), string(berr)
}
