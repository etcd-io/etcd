exec protoc --gogo_out=. -I=.:$GOPATH/src/code.google.com/p/gogoprotobuf/protobuf:$GOPATH/src *.proto
