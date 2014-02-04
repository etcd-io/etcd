package test

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// TestTLSOff asserts that non-TLS-encrypted communication between the
// etcd server and an unauthenticated client works
func TestTLSOff(t *testing.T) {
	proc, err := startServer([]string{})
	if err != nil {
		t.Fatal(err.Error())
	}
	defer stopServer(proc)

	client := buildClient()
	err = assertServerFunctional(client, "http")
	if err != nil {
		t.Fatal(err.Error())
	}
}

// TestTLSAnonymousClient asserts that TLS-encrypted communication between the etcd
// server and an anonymous client works
func TestTLSAnonymousClient(t *testing.T) {
	proc, err := startServer([]string{
		"-cert-file=../../fixtures/ca/server.crt",
		"-key-file=../../fixtures/ca/server.key.insecure",
	})
	if err != nil {
		t.Fatal(err.Error())
	}
	defer stopServer(proc)

	cacertfile := "../../fixtures/ca/ca.crt"

	cp := x509.NewCertPool()
	bytes, err := ioutil.ReadFile(cacertfile)
	if err != nil {
		panic(err)
	}
	cp.AppendCertsFromPEM(bytes)

	cfg := tls.Config{}
	cfg.RootCAs = cp

	client := buildTLSClient(&cfg)
	err = assertServerFunctional(client, "https")
	if err != nil {
		t.Fatal(err)
	}
}

// TestTLSAuthenticatedClient asserts that TLS-encrypted communication
// between the etcd server and an authenticated client works
func TestTLSAuthenticatedClient(t *testing.T) {
	proc, err := startServer([]string{
		"-cert-file=../../fixtures/ca/server.crt",
		"-key-file=../../fixtures/ca/server.key.insecure",
		"-ca-file=../../fixtures/ca/ca.crt",
	})
	if err != nil {
		t.Fatal(err.Error())
	}
	defer stopServer(proc)

	cacertfile := "../../fixtures/ca/ca.crt"
	certfile := "../../fixtures/ca/server2.crt"
	keyfile := "../../fixtures/ca/server2.key.insecure"

	cert, err := tls.LoadX509KeyPair(certfile, keyfile)
	if err != nil {
		panic(err)
	}

	cp := x509.NewCertPool()
	bytes, err := ioutil.ReadFile(cacertfile)
	if err != nil {
		panic(err)
	}
	cp.AppendCertsFromPEM(bytes)

	cfg := tls.Config{}
	cfg.Certificates = []tls.Certificate{cert}
	cfg.RootCAs = cp

	time.Sleep(time.Second)

	client := buildTLSClient(&cfg)
	err = assertServerFunctional(client, "https")
	if err != nil {
		t.Fatal(err)
	}
}

// TestTLSUnathenticatedClient asserts that TLS-encrypted communication
// between the etcd server and an unauthenticated client fails
func TestTLSUnauthenticatedClient(t *testing.T) {
	proc, err := startServer([]string{
		"-cert-file=../../fixtures/ca/server.crt",
		"-key-file=../../fixtures/ca/server.key.insecure",
		"-ca-file=../../fixtures/ca/ca.crt",
	})
	if err != nil {
		t.Fatal(err.Error())
	}
	defer stopServer(proc)

	cacertfile := "../../fixtures/ca/ca.crt"
	certfile := "../../fixtures/ca/broken/server.crt"
	keyfile := "../../fixtures/ca/broken/server.key.insecure"

	cert, err := tls.LoadX509KeyPair(certfile, keyfile)
	if err != nil {
		panic(err)
	}

	cp := x509.NewCertPool()
	bytes, err := ioutil.ReadFile(cacertfile)
	if err != nil {
		panic(err)
	}
	cp.AppendCertsFromPEM(bytes)

	cfg := tls.Config{}
	cfg.Certificates = []tls.Certificate{cert}
	cfg.RootCAs = cp

	time.Sleep(time.Second)

	client := buildTLSClient(&cfg)
	err = assertServerNotFunctional(client, "https")
	if err != nil {
		t.Fatal(err)
	}
}


func buildClient() http.Client {
	return http.Client{}
}

func buildTLSClient(tlsConf *tls.Config) http.Client {
	tr := http.Transport{TLSClientConfig: tlsConf}
	return http.Client{Transport: &tr}
}

func startServer(extra []string) (*os.Process, error) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	cmd := []string{"etcd",	"-f", "-data-dir=/tmp/node1", "-name=node1"}
	cmd = append(cmd, extra...)

	println(strings.Join(cmd, " "))

	return os.StartProcess(EtcdBinPath, cmd, procAttr)
}

func stopServer(proc *os.Process) {
	err := proc.Kill()
	if err != nil {
		panic(err.Error())
	}
	proc.Release()
}

func assertServerFunctional(client http.Client, scheme string) error {
	path := fmt.Sprintf("%s://127.0.0.1:4001/v2/keys/foo", scheme)
	fields := url.Values(map[string][]string{"value": []string{"bar"}})

	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)

		resp, err := client.PostForm(path, fields)
		if err == nil {
			if resp.StatusCode != 201 {
				return errors.New(fmt.Sprintf("resp.StatusCode == %s", resp.Status))
			} else {
				return nil
			}
		}
	}

	return errors.New("etcd server was not reachable in time")
}

func assertServerNotFunctional(client http.Client, scheme string) error {
	path := fmt.Sprintf("%s://127.0.0.1:4001/v2/keys/foo", scheme)
	fields := url.Values(map[string][]string{"value": []string{"bar"}})

	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)

		_, err := client.PostForm(path, fields)
		if err == nil {
			return errors.New("Expected error during POST, got nil")
		} else {
			errString := err.Error()
			if strings.Contains(errString, "connection refused") {
				continue
			} else if strings.Contains(errString, "bad certificate") {
				return nil
			} else {
				return err
			}
		}
	}

	return errors.New("Expected server to fail with 'bad certificate'")
}
