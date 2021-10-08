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
	binDir  string
	certDir string

	certPath       string
	privateKeyPath string
	caPath         string

	certPath2       string
	privateKeyPath2 string

	certPath3       string
	privateKeyPath3 string

	crlPath               string
	revokedCertPath       string
	revokedPrivateKeyPath string

	fixturesDir = integration.MustAbsPath("../fixtures")
)

func InitFlags() {
	os.Setenv("ETCD_UNSUPPORTED_ARCH", runtime.GOARCH)
	os.Unsetenv("ETCDCTL_API")

	binDirDef := integration.MustAbsPath("../../bin")
	certDirDef := fixturesDir

	flag.StringVar(&binDir, "bin-dir", binDirDef, "The directory for store etcd and etcdctl binaries.")
	flag.StringVar(&certDir, "cert-dir", certDirDef, "The directory for store certificate files.")
	flag.Parse()

	binPath = binDir + "/etcd"
	ctlBinPath = binDir + "/etcdctl"
	utlBinPath = binDir + "/etcdutl"
	certPath = certDir + "/server.crt"
	privateKeyPath = certDir + "/server.key.insecure"
	caPath = certDir + "/ca.crt"
	revokedCertPath = certDir + "/server-revoked.crt"
	revokedPrivateKeyPath = certDir + "/server-revoked.key.insecure"
	crlPath = certDir + "/revoke.crl"

	certPath2 = certDir + "/server2.crt"
	privateKeyPath2 = certDir + "/server2.key.insecure"

	certPath3 = certDir + "/server3.crt"
	privateKeyPath3 = certDir + "/server3.key.insecure"
}
