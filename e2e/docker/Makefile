# run makefile from repo root

docker-dns-build:
	docker build -t etcd-dns e2e/docker/

docker-dns-test: docker-dns-build
	docker run --dns 127.0.0.1 --rm -v `pwd`/bin/:/etcd -v `pwd`/integration/fixtures:/certs -w /etcd -t etcd-dns
