package transport

import (
	"io/ioutil"
	"os"
	"testing"
)

var (
	TLSCA = []byte(`-----BEGIN CERTIFICATE-----
MIIFNDCCAx6gAwIBAgIBATALBgkqhkiG9w0BAQUwLTEMMAoGA1UEBhMDVVNBMRAw
DgYDVQQKEwdldGNkLWNhMQswCQYDVQQLEwJDQTAeFw0xNDAzMTMwMjA5MDlaFw0y
NDAzMTMwMjA5MDlaMC0xDDAKBgNVBAYTA1VTQTEQMA4GA1UEChMHZXRjZC1jYTEL
MAkGA1UECxMCQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDdlBlw
Jiakc4C1UpMUvQ+2fttyBMfMLivQgj51atpKd8qIBvpZwz1wtpzdRG0hSYMF0IUk
MfBqyg+T5tt2Lfs3Gx3cYKS7G0HTfmABC7GdG8gNvEVNl/efxqvhis7p7hur765e
J+N2GR4oOOP5Wa8O5flv10cp3ZJLhAguc2CONLzfh/iAYAItFgktGHXJ/AnUhhaj
KWdKlK9Cv71YsRPOiB1hCV+LKfNSqrXPMvQ4sarz3yECIBhpV/KfskJoDyeNMaJd
gabX/S7gUCd2FvuOpGWdSIsDwyJf0tnYmQX5XIQwBZJib/IFMmmoVNYc1bFtYvRH
j0g0Ax4tHeXU/0mglqEcaTuMejnx8jlxZAM8Z94wHLfKbtaP0zFwMXkaM4nmfZqh
vLZwowDGMv9M0VRFEhLGYIc3xQ8G2u8cFAGw1UqTxKhwAdRmrcFaQ38sk4kziy0u
AkpGavS7PKcFjjB/fdDFO/kwGQOthX/oTn9nP3BT+IK2h1A6ATMPI4lVnhb5/KBt
9M/fGgbiU+I9QT0Ilz/LlrcCuzyRXREvIZvoUL77Id+JT3qQxqPn/XMKLN4WEFII
112MFGqCD85JZzNoC4RkZd8kFlR4YJWsS4WqJlWprESr5cCDuLviK+31cnIRF4fJ
mz0gPsVgY7GFEan3JJnL8oRUVzdTPKfPt0atsQIDAQABo2MwYTAOBgNVHQ8BAf8E
BAMCAAQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUnVlVvktY+zlLpG43nTpG
AWmUkrYwHwYDVR0jBBgwFoAUnVlVvktY+zlLpG43nTpGAWmUkrYwCwYJKoZIhvcN
AQEFA4ICAQAqIcPFux3V4h1N0aGM4fCS/iT50TzDnRb5hwILKbmyA6LFnH4YF7PZ
aA0utDNo1XSRDMpR38HWk0weh5Sfx6f2danaKZHAsea8oVEtdrz16ZMOvoh0CPIM
/hn0CGQOoXDADDNFASuExhhpoyYkDqTVTCQ/zbhZg1mjBljJ+BBzlSgeoE4rUDpn
nuDcmD9LtjpsVQL+J662rd51xV4Z6a7aZLvN9GfO8tYkfCGCD9+fGh1Cpz0IL7qw
VRie+p/XpjoHemswnRhYJ4wn10a1UkVSR++wld6Gvjb9ikyr9xVyU5yrRM55pP2J
VguhzjhTIDE1eDfIMMxv3Qj8+BdVQwtKFD+zQYQcbcjsvjTErlS7oCbM2DVlPnRT
QaCM0q0yorfzc4hmml5P95ngz2xlohavgNMhsYIlcWyq3NVbm7mIXz2pjqa16Iit
vL7WX6OVupv/EOMRx5cVcLqqEaYJmAlNd/CCD8ihDQCwoJ6DJhczPRexrVp+iZHK
SnIUONdXb/g8ungXUGL1jGNQrWuq49clpI5sLWNjMDMFAQo0qu5bLkOIMlK/evCt
gctOjXDvGXCk5h6Adf14q9zDGFdLoxw0/aciUSn9IekdzYPmkYUTifuzkVRsPKzS
nmI4dQvz0rHIh4FBUKWWrJhRWhrv9ty/YFuJXVUHeAwr5nz6RFZ4wQ==
-----END CERTIFICATE-----`)
	TLSKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`)
	TLSCert = []byte(`-----BEGIN CERTIFICATE-----
MIIFWzCCA0WgAwIBAgIBAjALBgkqhkiG9w0BAQUwLTEMMAoGA1UEBhMDVVNBMRAw
DgYDVQQKEwdldGNkLWNhMQswCQYDVQQLEwJDQTAeFw0xNDAzMTMwMjA5MjJaFw0y
NDAzMTMwMjA5MjJaMEUxDDAKBgNVBAYTA1VTQTEQMA4GA1UEChMHZXRjZC1jYTEP
MA0GA1UECxMGc2VydmVyMRIwEAYDVQQDEwkxMjcuMC4wLjEwggIiMA0GCSqGSIb3
DQEBAQUAA4ICDwAwggIKAoICAQDI3EvqJrLWsnPbjAT8ENiMRyBINhhafubi5Nb+
glEzkbC2kv2zXkVkpkBubDRwyh3eomSbdwKYk3yz+IopT753teJueRpMPq9Ayr/+
PZl4Y1tG04KcjfOvOls6zPsDfHzluR8TE705If5wwZu3Bdwxzdtx9T0ROzIEgRt0
Axuce5qkg93IWNxOrIr+4LCxYfTpvpTXO20lz0IuQNm1Opo9PVoWn7PXdOmuCzSG
2hW1DcKqSyQP7IkplBJS0EhoovIsXavSkPKJssvQj73ZFIBVgKhXuHmPNdrypaQk
CtxsqbVdOOlojItqYTTDAiadwRQWkYgDOSQCGJiPqYVJx+rH4MlzxQ6n9x2qIcne
lfMr+VFDEc1YvHu1XLMg5b1ImD6ChutYW0RhFJ3CQVdQR2i4kJ8T1DSJYLISMODZ
ux1cZaUoSL/EkrC5/8POWZmP8nJXO6A4wrZDHF30/qWpo+T5PvsA6cABfX1jkcTx
PBXGK1qOZ8rToTxprJ2zc3zuZNxSgM32nzjcPUgn559Mgdl0HR4c4JeTZGsebWmx
MWmkz//BV4eUaGHqCpzRQHf3YIxysvDC2Xf4z2Alk8AlLRXp7/ksatdxAtyc+y8+
MWCc6N0YbI9zjv+ezCBqR+mu1P5Tb0HebPFz3dOdIpiC3kU8QyMEagw8u5xliZs4
AxwdNwIDAQABo3IwcDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwHQYD
VR0OBBYEFD6UrVN8uolWz6et79jVeZetjd4XMB8GA1UdIwQYMBaAFJ1ZVb5LWPs5
S6RuN506RgFplJK2MA8GA1UdEQQIMAaHBH8AAAEwCwYJKoZIhvcNAQEFA4ICAQCo
sKn1Rjx0tIVWAZAZB4lCWvkQDp/txnb5zzQUlKhIW2o98IklASmOYYyZbE2PXlda
/n8TwKIzWgIoNh5AcgLWhtASrnZdGFXY88n5jGk6CVZ1+Dl+IX99h+r+YHQzf1jU
BjGrZHGv3pPjwhFGDS99lM/TEBk/eLI2Kx5laL+nWMTwa8M1OwSIh6ZxYPVlWUqb
rurk5l/YqW+UkYIXIQhe6LwtB7tBjr6nDIWBfHQ7uN8IdB8VIAF6lejr22VmERTW
j+zJ5eTzuQN1f0s930mEm8pW7KgGxlEqrUlSJtxlMFCv6ZHZk1Y4yEiOCBKlPNme
X3B+lhj//PH3gLNm3+ZRr5ena3k+wL9Dd3d3GDCIx0ERQyrGS/rJpqNPI+8ZQlG0
nrFlm7aP6UznESQnJoSFbydiD0EZ4hXSdmDdXQkTklRpeXfMcrYBGN7JrGZOZ2T2
WtXBMx2bgPeEH50KRrwUMFe122bchh0Fr+hGvNK2Q9/gRyQPiYHq6vSF4GzorzLb
aDuWA9JRH8/c0z8tMvJ7KjmmmIxd39WWGZqiBrGQR7utOJjpQl+HCsDIQM6yZ/Bu
RpwKj2yBz0OQg4tWbtqUuFkRMTkCR6vo3PadgO1VWokM7UFUXlScnYswcM5EwnzJ
/IsYJ2s1V706QVUzAGIbi3+wYi3enk7JfYoGIqa2oA==
-----END CERTIFICATE-----`)
)

func createTempFile(b []byte) (string, error) {
	f, err := ioutil.TempFile("", "etcd-test-tls-")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err = f.Write(b); err != nil {
		return "", err
	}

	return f.Name(), nil
}

func TestNewTransportTLSInfo(t *testing.T) {
	fCA, err := createTempFile(TLSCA)
	if err != nil {
		t.Fatalf("Unable to prepare TLS CA tmpfile: %v", err)
	}
	defer os.Remove(fCA)

	fCert, err := createTempFile(TLSCert)
	if err != nil {
		t.Fatalf("Unable to prepare TLS cert tmpfile: %v", err)
	}
	defer os.Remove(fCert)

	fKey, err := createTempFile(TLSKey)
	if err != nil {
		t.Fatalf("Unable to prepare TLS key tmpfile: %v", err)
	}
	defer os.Remove(fKey)

	tests := []struct {
		info                TLSInfo
		wantTLSClientConfig bool
	}{
		{
			info:                TLSInfo{},
			wantTLSClientConfig: false,
		},
		{
			info: TLSInfo{
				CertFile: fCert,
				KeyFile:  fKey,
			},
			wantTLSClientConfig: true,
		},
		{
			info: TLSInfo{
				CertFile: fCert,
				KeyFile:  fKey,
				CAFile:   fCA,
			},
			wantTLSClientConfig: true,
		},
	}

	for i, tt := range tests {
		trans, err := NewTransport(tt.info)
		if err != nil {
			t.Fatalf("Received unexpected error from NewTransport: %v", err)
		}

		gotTLSClientConfig := trans.TLSClientConfig != nil
		if tt.wantTLSClientConfig != gotTLSClientConfig {
			t.Fatalf("%#d: wantTLSClientConfig=%t but gotTLSClientConfig=%t", i, tt.wantTLSClientConfig, gotTLSClientConfig)
		}
	}
}
