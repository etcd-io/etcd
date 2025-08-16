package agent

import (
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type GlobalConfig struct {
	Endpoints           []string `json:"endpoints,omitempty"`
	UseClusterEndpoints bool     `json:"useClusterEndpoints,omitempty"`

	DialTimeout      time.Duration `json:"dial-timeout,omitempty"`
	CommandTimeout   time.Duration `json:"command-timeout,omitempty"`
	KeepAliveTime    time.Duration `json:"keep-alive-time,omitempty"`
	KeepAliveTimeout time.Duration `json:"keep-alive-timeout,omitempty"`

	Insecure           bool `json:"insecure,omitempty"`
	InsecureDiscovery  bool `json:"insecure-discovery,omitempty"`
	InsecureSkipVerify bool `json:"insecure-skip-verify,omitempty"`

	CertFile string `json:"cert-file,omitempty"`
	KeyFile  string `json:"key-file,omitempty"`
	CaFile   string `json:"ca-file,omitempty"`

	DNSDomain  string `json:"dns-domain,omitempty"`
	DNSService string `json:"dns-service,omitempty"`

	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	DbQuotaBytes int `json:"db-quota-bytes,omitempty"`

	PrintVersion bool `json:"print-version,omitempty"`

	Offline bool   `json:"offline,omitempty"`
	DataDir string `json:"data-dir,omitempty"`
}

func clientConfigWithoutEndpoints(gcfg GlobalConfig) *clientv3.ConfigSpec {
	cfg := &clientv3.ConfigSpec{
		DialTimeout:      gcfg.DialTimeout,
		KeepAliveTime:    gcfg.KeepAliveTime,
		KeepAliveTimeout: gcfg.KeepAliveTimeout,

		Secure: secureConfig(gcfg),
		Auth:   authConfig(gcfg),
	}

	return cfg
}

func secureConfig(gcfg GlobalConfig) *clientv3.SecureConfig {
	scfg := &clientv3.SecureConfig{
		Cert:       gcfg.CertFile,
		Key:        gcfg.KeyFile,
		Cacert:     gcfg.CaFile,
		ServerName: gcfg.DNSDomain,

		InsecureTransport:  gcfg.Insecure,
		InsecureSkipVerify: gcfg.InsecureSkipVerify,
	}

	if gcfg.InsecureDiscovery {
		scfg.ServerName = ""
	}

	return scfg
}

func authConfig(gcfg GlobalConfig) *clientv3.AuthConfig {
	if gcfg.Username == "" {
		return nil
	}

	if gcfg.Password == "" {
		userSecret := strings.SplitN(gcfg.Username, ":", 2)
		if len(userSecret) < 2 {
			return nil
		}

		return &clientv3.AuthConfig{
			Username: userSecret[0],
			Password: userSecret[1],
		}
	}

	return &clientv3.AuthConfig{
		Username: gcfg.Username,
		Password: gcfg.Password,
	}
}
