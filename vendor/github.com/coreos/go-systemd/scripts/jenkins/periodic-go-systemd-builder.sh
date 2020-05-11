#!/bin/bash

echo ""
echo "Public IP: "`wget -O - http://checkip.amazonaws.com/ 2>/dev/null || true`
echo "Systemd version: "`systemctl --version || true`
echo "Linux version: "`uname -srm || true`
echo "Distribution version: "`(. /etc/os-release ; echo $PRETTY_NAME) || true`
echo ""
echo "git commit: "`git describe --abbrev=40 HEAD || true`
echo ""

# unsquashfs is in /usr/sbin
export PATH=/usr/lib64/ccache:/usr/local/bin:/usr/bin:/bin:/usr/local/sbin:/usr/sbin:/home/fedora/.local/bin:/home/fedora/bin

# As a convenience, set up a self-contained GOPATH if none set
if [ ! -h gopath/src/github.com/coreos/go-systemd ]; then
    mkdir -p gopath/src/github.com/coreos
    ln -s ../../../.. gopath/src/github.com/coreos/go-systemd || exit 255
fi
export GOPATH=${PWD}/gopath
go get -u github.com/godbus/dbus
go get github.com/coreos/pkg/dlopen

sudo -E ./test
