version=$(git describe --tags --always)
go build -ldflags "-X main.version 'etcd $version'" "$@"
