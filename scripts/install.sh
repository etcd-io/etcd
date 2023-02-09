#!/usr/bin/env bash

install_direct () {
    # Figure out the arch.
	uname="$(uname -s)"
	case "${uname}" in
		Linux*)     machine=linux;;
		Darwin*)    machine=darwin;;
		*)          
            machine="UNKNOWN:${uname}"
            echo "Cannot install on invalid machine ${machine}"
            exit 1
	esac

	echo "Installing for ${machine}"
    # choose either URL
    GOOGLE_URL=https://storage.googleapis.com/etcd
    # GITHUB_URL=https://github.com/etcd-io/etcd/releases/download
    DOWNLOAD_URL=${GOOGLE_URL}

	# cleanup
    rm -f "/tmp/etcd-${ETCD_VER}-${machine}-amd64.tar.gz"
    rm -rf /tmp/etcd-download-test && mkdir -p /tmp/etcd-download-test

	# install 
    if [ "$machine" == "linux" ]; then
        curl -L "${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-${machine}-amd64.tar.gz" -o "/tmp/etcd-${ETCD_VER}-${machine}-amd64.tar.gz"
        tar xzvf "/tmp/etcd-${ETCD_VER}-${machine}-amd64.tar.gz" -C "/tmp/etcd-download-test --strip-components=1"
        rm -f "/tmp/etcd-${ETCD_VER}-${machine}-amd64.tar.gz"
    else
		curl -L "${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-${machine}-amd64.zip" -o "/tmp/etcd-${ETCD_VER}-${machine}-amd64.zip"
		unzip "/tmp/etcd-${ETCD_VER}-${machine}-amd64.zip" -d /tmp && rm -f "/tmp/etcd-${ETCD_VER}-${machine}-amd64.zip"
		mv /tmp/etcd-"${ETCD_VER}"-"${machine}"-amd64/* /tmp/etcd-download-test && rm -rf mv "/tmp/etcd-${ETCD_VER}-${machine}-amd64"
    fi

	# test
    /tmp/etcd-download-test/etcd --version
    /tmp/etcd-download-test/etcdctl version
    /tmp/etcd-download-test/etcdutl version

    # start a local etcd server
    /tmp/etcd-download-test/etcd

    # write,read to etcd
    /tmp/etcd-download-test/etcdctl --endpoints=localhost:2379 put foo bar
    /tmp/etcd-download-test/etcdctl --endpoints=localhost:2379 get foo
}

install_docker() {
	rm -rf /tmp/etcd-data.tmp && mkdir -p /tmp/etcd-data.tmp && \
		docker rmi gcr.io/etcd-development/etcd:"${ETCD_VER}" || true && \
		docker run \
		-p 2379:2379 \
		-p 2380:2380 \
		--mount type=bind,source=/tmp/etcd-data.tmp,destination=/etcd-data \
		--name etcd-gcr-"${ETCD_VER}" \
		gcr.io/etcd-development/etcd:"${ETCD_VER}" \
		/usr/local/bin/etcd \
		--name s1 \
		--data-dir /etcd-data \
		--listen-client-urls http://0.0.0.0:2379 \
		--advertise-client-urls http://0.0.0.0:2379 \
		--listen-peer-urls http://0.0.0.0:2380 \
		--initial-advertise-peer-urls http://0.0.0.0:2380 \
		--initial-cluster s1=http://0.0.0.0:2380 \
		--initial-cluster-token tkn \
		--initial-cluster-state new \
		--log-level info \
		--logger zap \
		--log-outputs stderr

    # Remove the following ? we never reach this part as docker run spawns an etcd daemon
	docker exec etcd-gcr-"${ETCD_VER}" /bin/sh -c "/usr/local/bin/etcd --version"
	docker exec etcd-gcr-"${ETCD_VER}" /bin/sh -c "/usr/local/bin/etcdctl version"
	docker exec etcd-gcr-"${ETCD_VER}" /bin/sh -c "/usr/local/bin/etcdutl version"
	docker exec etcd-gcr-"${ETCD_VER}" /bin/sh -c "/usr/local/bin/etcdctl endpoint health"
	docker exec etcd-gcr-"${ETCD_VER}" /bin/sh -c "/usr/local/bin/etcdctl put foo bar"
	docker exec etcd-gcr-"${ETCD_VER}" /bin/sh -c "/usr/local/bin/etcdctl get foo"
}

if [ "$#" -lt 1 ]; then
    echo "Illegal number of parameters"
    echo "Usage: $0 <version> <docker>"
    echo "e.g. of <version> : 3.5.6"
    echo "<docker> parameter is optional"
    exit 1
fi

ETCD_VER="v""$1"
if [ "$2" == "docker" ]; then
    install_docker
else
    install_direct
fi
