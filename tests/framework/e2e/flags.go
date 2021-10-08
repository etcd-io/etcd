// Copyright 2021 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"flag"
	"os"
	"runtime"

	"go.etcd.io/etcd/tests/v3/integration"
)

var (
	BinDir  string
	CertDir string

	CertPath       string
	PrivateKeyPath string
	CaPath         string

	CertPath2       string
	PrivateKeyPath2 string

	CertPath3       string
	PrivateKeyPath3 string

	CrlPath               string
	RevokedCertPath       string
	RevokedPrivateKeyPath string

	FixturesDir = integration.MustAbsPath("../fixtures")
)

func InitFlags() {
	os.Setenv("ETCD_UNSUPPORTED_ARCH", runtime.GOARCH)
	os.Unsetenv("ETCDCTL_API")

	binDirDef := integration.MustAbsPath("../../bin")
	certDirDef := FixturesDir

	flag.StringVar(&BinDir, "bin-dir", binDirDef, "The directory for store etcd and etcdctl binaries.")
	flag.StringVar(&CertDir, "cert-dir", certDirDef, "The directory for store certificate files.")
	flag.Parse()

	BinPath = BinDir + "/etcd"
	CtlBinPath = BinDir + "/etcdctl"
	UtlBinPath = BinDir + "/etcdutl"
	CertPath = CertDir + "/server.crt"
	PrivateKeyPath = CertDir + "/server.key.insecure"
	CaPath = CertDir + "/ca.crt"
	RevokedCertPath = CertDir + "/server-revoked.crt"
	RevokedPrivateKeyPath = CertDir + "/server-revoked.key.insecure"
	CrlPath = CertDir + "/revoke.crl"

	CertPath2 = CertDir + "/server2.crt"
	PrivateKeyPath2 = CertDir + "/server2.key.insecure"

	CertPath3 = CertDir + "/server3.crt"
	PrivateKeyPath3 = CertDir + "/server3.key.insecure"
}
