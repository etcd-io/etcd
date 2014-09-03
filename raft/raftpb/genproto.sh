set -e
protoc --gogo_out=. -I=.:$GOPATH/src/code.google.com/p/gogoprotobuf/protobuf:$GOPATH/src *.proto

prefix=github.com/coreos/etcd/third_party
sed \
	-i'.bak' \
	"s|code.google.com/p/gogoprotobuf/proto|$prefix/code.google.com/p/gogoprotobuf/proto|" *.go
rm *.bak
