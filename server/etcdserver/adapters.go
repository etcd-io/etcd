package etcdserver

import (
	"context"

	"github.com/coreos/go-semver/semver"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	serverversion "go.etcd.io/etcd/server/v3/etcdserver/version"
)

// serverVersionAdapter implements Server interface needed by serverversion.Monitor
type serverVersionAdapter struct {
	*EtcdServer
}

var _ serverversion.Server = (*serverVersionAdapter)(nil)

func (s *serverVersionAdapter) UpdateClusterVersion(version string) {
	// TODO switch to updateClusterVersionV3 in 3.6
	s.GoAttach(func() { s.updateClusterVersionV2(version) })
}

func (s *serverVersionAdapter) DowngradeCancel() {
	ctx, cancel := context.WithTimeout(context.Background(), s.Cfg.ReqTimeout())
	if _, err := s.downgradeCancel(ctx); err != nil {
		s.lg.Warn("failed to cancel downgrade", zap.Error(err))
	}
	cancel()
}

func (s *serverVersionAdapter) GetClusterVersion() *semver.Version {
	return s.cluster.Version()
}

func (s *serverVersionAdapter) GetDowngradeInfo() *membership.DowngradeInfo {
	return s.cluster.DowngradeInfo()
}

func (s *serverVersionAdapter) GetVersions() map[string]*version.Versions {
	return getVersions(s.lg, s.cluster, s.id, s.peerRt)
}
