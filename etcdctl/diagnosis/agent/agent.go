package agent

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func MemberList(gcfg GlobalConfig, eps []string, options ...clientv3.OpOption) (*clientv3.MemberListResponse, error) {
	cfgSpec := clientConfigWithoutEndpoints(gcfg)
	var err error
	if len(eps) == 0 {
		eps, err = endpointsFromCmd(gcfg)
		if err != nil {
			return nil, err
		}
	}

	cfgSpec.Endpoints = eps

	c, err := createClient(cfgSpec)
	if err != nil {
		return nil, err
	}

	ctx, cancel := commandCtx(gcfg.CommandTimeout)
	defer func() {
		c.Close()
		cancel()
	}()

	members, err := c.MemberList(ctx, options...)
	if err != nil {
		return nil, err
	}

	return members, nil
}

func EndpointStatus(gcfg GlobalConfig, ep string) (*clientv3.StatusResponse, error) {
	cfgSpec := clientConfigWithoutEndpoints(gcfg)
	cfgSpec.Endpoints = []string{ep}
	c, err := createClient(cfgSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to createClient: %w", err)
	}

	ctx, cancel := commandCtx(gcfg.CommandTimeout)
	defer func() {
		c.Close()
		cancel()
	}()
	return c.Status(ctx, ep)
}

func Read(gcfg GlobalConfig, eps []string, key string, options ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	cfgSpec := clientConfigWithoutEndpoints(gcfg)
	cfgSpec.Endpoints = eps
	c, err := createClient(cfgSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to createClient: %w", err)
	}

	ctx, cancel := commandCtx(gcfg.CommandTimeout)
	defer func() {
		c.Close()
		cancel()
	}()

	return c.Get(ctx, key, options...)
}

func Metrics(gcfg GlobalConfig, ep string) ([]string, error) {
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		ep = "http://" + ep
	}
	urlPath, err := url.JoinPath(ep, "metrics")
	if err != nil {
		return nil, fmt.Errorf("failed to join metrics url path: %w", err)
	}

	client := &http.Client{}
	if strings.HasPrefix(urlPath, "https://") {
		// load client certificate
		cert, err := tls.LoadX509KeyPair(gcfg.CertFile, gcfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}
		// load CA
		caCert, err := os.ReadFile(gcfg.CaFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA: %w", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				RootCAs:            caCertPool,
				InsecureSkipVerify: gcfg.InsecureSkipVerify,
			},
		}
		client.Transport = tr
	}
	resp, err := client.Get(urlPath)
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metrics response: %w", err)
	}

	return strings.Split(string(data), "\n"), nil
}
