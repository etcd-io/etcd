package etcd

import (
	"os"
	"time"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

func registerAvailableInternalVersions(internalDir, name, addr string, tls *server.TLSInfo) {
	var c *etcd.Client
	if tls.Scheme() == "http" {
		c = etcd.NewClient([]string{addr})
	} else {
		var err error
		c, err = etcd.NewTLSClient([]string{addr}, tls.CertFile, tls.KeyFile, tls.CAFile)
		if err != nil {
			log.Fatalf("client TLS error: %v", err)
		}
	}

	vers, err := getInternalVersions(internalDir)
	if err != nil {
		log.Infof("failed to get local etcd versions: %v", err)
		return
	}
	for _, v := range vers {
		for {
			_, err := c.Set("/_etcd/available-internal-versions/"+v+"/"+name, "ok", 0)
			if err == nil {
				break
			}
			time.Sleep(time.Second)
		}
	}
	log.Infof("%s: available_internal_versions %s is registered into key space successfully.", name, vers)
}

func getInternalVersions(internalDir string) ([]string, error) {
	dir, err := os.Open(internalDir)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return dir.Readdirnames(-1)
}
