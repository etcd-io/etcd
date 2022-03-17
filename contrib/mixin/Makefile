.PHONY: tools manifests test clean

OS := linux
ARCH ?= amd64
PROMETHEUS_VERSION := 2.33.1

tools:
	go install github.com/google/go-jsonnet/cmd/jsonnet@latest
	go install github.com/brancz/gojsontoyaml@latest
	wget -qO- "https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.${OS}-${ARCH}.tar.gz" |\
	tar xvz --strip-components=1 -C "$$(go env GOPATH)/bin" prometheus-${PROMETHEUS_VERSION}.${OS}-${ARCH}/promtool

manifests: manifests/etcd-prometheusRules.yaml

manifests/etcd-prometheusRules.yaml:
	mkdir -p manifests
	jsonnet -e '(import "mixin.libsonnet").prometheusAlerts' | gojsontoyaml > manifests/etcd-prometheusRules.yaml

test: manifests/etcd-prometheusRules.yaml
	promtool test rules test.yaml

clean:
	rm -rf manifests/*.yaml
