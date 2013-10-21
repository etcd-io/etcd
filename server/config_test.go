package server

import (
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
	assert.Equal(t, c.CPUProfileFile, "XXX", "")
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
