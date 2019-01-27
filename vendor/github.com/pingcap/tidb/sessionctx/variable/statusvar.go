// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package variable

import (
	"bytes"
	"crypto/tls"

	"github.com/pingcap/errors"
)

var statisticsList []Statistics

// DefaultStatusVarScopeFlag is the default scope of status variables.
var DefaultStatusVarScopeFlag = ScopeGlobal | ScopeSession

// StatusVal is the value of the corresponding status variable.
type StatusVal struct {
	Scope ScopeFlag
	Value interface{}
}

// Statistics is the interface of statistics.
type Statistics interface {
	// GetScope gets the status variables scope.
	GetScope(status string) ScopeFlag
	// Stats returns the statistics status variables.
	Stats(*SessionVars) (map[string]interface{}, error)
}

// RegisterStatistics registers statistics.
func RegisterStatistics(s Statistics) {
	statisticsList = append(statisticsList, s)
}

// GetStatusVars gets registered statistics status variables.
// TODO: Refactor this function to avoid repeated memory allocation / dealloc
func GetStatusVars(vars *SessionVars) (map[string]*StatusVal, error) {
	statusVars := make(map[string]*StatusVal)

	for _, statistics := range statisticsList {
		vals, err := statistics.Stats(vars)
		if err != nil {
			return nil, errors.Trace(err)
		}

		for name, val := range vals {
			scope := statistics.GetScope(name)
			statusVars[name] = &StatusVal{Value: val, Scope: scope}
		}
	}

	return statusVars, nil
}

// Taken from https://golang.org/pkg/crypto/tls/#pkg-constants .
var tlsCiphers = []uint16{
	tls.TLS_RSA_WITH_RC4_128_SHA,
	tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
}

var tlsSupportedCiphers string

// Taken from https://github.com/openssl/openssl/blob/c784a838e0947fcca761ee62def7d077dc06d37f/include/openssl/ssl.h#L141 .
var tlsVersionString = map[uint16]string{
	tls.VersionSSL30: "SSLv3",
	tls.VersionTLS10: "TLSv1",
	tls.VersionTLS11: "TLSv1.1",
	tls.VersionTLS12: "TLSv1.2",
}

// Taken from https://testssl.sh/openssl-rfc.mapping.html .
var tlsCipherString = map[uint16]string{
	tls.TLS_RSA_WITH_RC4_128_SHA:                "RC4-SHA",
	tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA:           "DES-CBC3-SHA",
	tls.TLS_RSA_WITH_AES_128_CBC_SHA:            "AES128-SHA",
	tls.TLS_RSA_WITH_AES_256_CBC_SHA:            "AES256-SHA",
	tls.TLS_RSA_WITH_AES_128_CBC_SHA256:         "AES128-SHA256",
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256:         "AES128-GCM-SHA256",
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384:         "AES256-GCM-SHA384",
	tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA:        "ECDHE-ECDSA-RC4-SHA",
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:    "ECDHE-ECDSA-AES128-SHA",
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:    "ECDHE-ECDSA-AES256-SHA",
	tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA:          "ECDHE-RSA-RC4-SHA",
	tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA:     "ECDHE-RSA-DES-CBC3-SHA",
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:      "ECDHE-RSA-AES128-SHA",
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:      "ECDHE-RSA-AES256-SHA",
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256: "ECDHE-ECDSA-AES128-SHA256",
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256:   "ECDHE-RSA-AES128-SHA256",
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:   "ECDHE-RSA-AES128-GCM-SHA256",
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256: "ECDHE-ECDSA-AES128-GCM-SHA256",
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:   "ECDHE-RSA-AES256-GCM-SHA384",
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384: "ECDHE-ECDSA-AES256-GCM-SHA384",
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305:    "ECDHE-RSA-CHACHA20-POLY1305",
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305:  "ECDHE-ECDSA-CHACHA20-POLY1305",
}

var defaultStatus = map[string]*StatusVal{
	"Ssl_cipher":      {ScopeGlobal | ScopeSession, ""},
	"Ssl_cipher_list": {ScopeGlobal | ScopeSession, ""},
	"Ssl_verify_mode": {ScopeGlobal | ScopeSession, 0},
	"Ssl_version":     {ScopeGlobal | ScopeSession, ""},
}

type defaultStatusStat struct {
}

func (s defaultStatusStat) GetScope(status string) ScopeFlag {
	return defaultStatus[status].Scope
}

func (s defaultStatusStat) Stats(vars *SessionVars) (map[string]interface{}, error) {
	statusVars := make(map[string]interface{})

	for name, v := range defaultStatus {
		statusVars[name] = v.Value
	}

	// `vars` may be nil in unit tests.
	if vars != nil && vars.TLSConnectionState != nil {
		statusVars["Ssl_cipher"] = tlsCipherString[vars.TLSConnectionState.CipherSuite]
		statusVars["Ssl_cipher_list"] = tlsSupportedCiphers
		// tls.VerifyClientCertIfGiven == SSL_VERIFY_PEER | SSL_VERIFY_CLIENT_ONCE
		statusVars["Ssl_verify_mode"] = 0x01 | 0x04
		statusVars["Ssl_version"] = tlsVersionString[vars.TLSConnectionState.Version]
	}

	return statusVars, nil
}

func init() {
	var ciphersBuffer bytes.Buffer
	for _, v := range tlsCiphers {
		ciphersBuffer.WriteString(tlsCipherString[v])
		ciphersBuffer.WriteString(":")
	}
	tlsSupportedCiphers = ciphersBuffer.String()

	var stat defaultStatusStat
	RegisterStatistics(stat)
}
