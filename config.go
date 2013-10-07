/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path/filepath"
)

//--------------------------------------
// Config
//--------------------------------------

// Get the server info from previous conf file
// or from the user
func getInfo(path string) *Info {

	infoPath := filepath.Join(path, "info")

	if force {
		// Delete the old configuration if exist
		logPath := filepath.Join(path, "log")
		confPath := filepath.Join(path, "conf")
		snapshotPath := filepath.Join(path, "snapshot")
		os.Remove(infoPath)
		os.Remove(logPath)
		os.Remove(confPath)
		os.RemoveAll(snapshotPath)
	} else if info := readInfo(infoPath); info != nil {
		infof("Found node configuration in '%s'. Ignoring flags", infoPath)
		return info
	}

	// Read info from command line
	info := &argInfo

	// Write to file.
	content, _ := json.MarshalIndent(info, "", " ")
	content = []byte(string(content) + "\n")
	if err := ioutil.WriteFile(infoPath, content, 0644); err != nil {
		fatalf("Unable to write info to file: %v", err)
	}

	infof("Wrote node configuration to '%s'", infoPath)

	return info
}

// readInfo reads from info file and decode to Info struct
func readInfo(path string) *Info {
	file, err := os.Open(path)

	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		fatal(err)
	}
	defer file.Close()

	info := &Info{}

	content, err := ioutil.ReadAll(file)
	if err != nil {
		fatalf("Unable to read info: %v", err)
		return nil
	}

	if err = json.Unmarshal(content, &info); err != nil {
		fatalf("Unable to parse info: %v", err)
		return nil
	}

	return info
}

func tlsConfigFromInfo(info TLSInfo) (t TLSConfig, ok bool) {
	var keyFile, certFile, CAFile string
	var tlsCert tls.Certificate
	var err error

	t.Scheme = "http"

	keyFile = info.KeyFile
	certFile = info.CertFile
	CAFile = info.CAFile

	// If the user do not specify key file, cert file and
	// CA file, the type will be HTTP
	if keyFile == "" && certFile == "" && CAFile == "" {
		return t, true
	}

	// both the key and cert must be present
	if keyFile == "" || certFile == "" {
		return t, false
	}

	tlsCert, err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		fatal(err)
	}

	t.Scheme = "https"
	t.Server.ClientAuth, t.Server.ClientCAs = newCertPool(CAFile)

	// The client should trust the RootCA that the Server uses since
	// everyone is a peer in the network.
	t.Client.Certificates = []tls.Certificate{tlsCert}
	t.Client.RootCAs = t.Server.ClientCAs

	return t, true
}

// newCertPool creates x509 certPool and corresponding Auth Type.
// If the given CAfile is valid, add the cert into the pool and verify the clients'
// certs against the cert in the pool.
// If the given CAfile is empty, do not verify the clients' cert.
// If the given CAfile is not valid, fatal.
func newCertPool(CAFile string) (tls.ClientAuthType, *x509.CertPool) {
	if CAFile == "" {
		return tls.NoClientCert, nil
	}
	pemByte, err := ioutil.ReadFile(CAFile)
	check(err)

	block, pemByte := pem.Decode(pemByte)

	cert, err := x509.ParseCertificate(block.Bytes)
	check(err)

	certPool := x509.NewCertPool()

	certPool.AddCert(cert)

	return tls.RequireAndVerifyClientCert, certPool
}
