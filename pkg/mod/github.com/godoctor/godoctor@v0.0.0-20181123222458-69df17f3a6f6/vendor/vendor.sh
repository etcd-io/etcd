#!/bin/bash

# Vendoring of third-party packages for the Go Doctor

set -e

if [ `dirname $0` != '.' ]; then
	echo "vendor.sh must be run from the vendor directory"
	exit 1
fi

# Change to the root of the godoctor repository
#cd ..

FILE=`pwd`/versions.txt

echo $PWD

echo "Cloning repositories..."
rm -rf github.com/cheggaaa/pb && \
	git clone https://github.com/cheggaaa/pb.git github.com/cheggaaa/pb
rm -rf github.com/mattn/go-runewidth && \
	git clone https://github.com/mattn/go-runewidth.git github.com/mattn/go-runewidth
rm -rf github.com/willf/bitset && \
	git clone https://github.com/willf/bitset.git github.com/willf/bitset
rm -rf golang.org/x/sys && \
	git clone https://github.com/golang/sys.git golang.org/x/sys
rm -rf golang.org/x/tools && \
	git clone https://github.com/golang/tools.git golang.org/x/tools

echo "Logging versions to $FILE and removing .git directories..."
date >$FILE
for pkg in \
		./github.com/cheggaaa/pb \
		./github.com/mattn/go-runewidth \
		./github.com/willf/bitset \
		./golang.org/x/sys \
		./golang.org/x/tools
do
	pushd . >/dev/null
	cd $pkg
	echo "" >>$FILE
	echo "$pkg" >>$FILE
	git remote -v | head -1 >>$FILE
	git log --pretty=format:'%H %d %s' -1 >>$FILE
	echo "" >>$FILE
	rm -rf ".git"
	popd >/dev/null
done

echo "Removing unused portions of go.tools..."
pushd . >/dev/null
cd golang.org/x/tools && \
	rm -rf blog cmd cover dashboard godoc imports oracle playground \
		present refactor codereview.cfg go/callgraph \
		go/gcimporter go/gccgoimporter \
		go/importer go/pointer go/ssa go/vcs
popd >/dev/null

echo "Removing tests from third-party packages..."
find . -iname '*_test.go' -delete

echo "DONE"
exit 0
