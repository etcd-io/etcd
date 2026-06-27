
# protodoc

[![Build Status](https://img.shields.io/travis/etcd-io/protodoc.svg?style=flat-square)][cistat] [![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)][protodoc-godoc]

protodoc generates Protocol Buffer documentation.

```
go get -v -u go.etcd.io/protodoc

protodoc --directory=./parse/testdata \
	--parse="service,message" \
	--languages="Go,C++,Java,Python" \
	--title=testdata \
	--output=sample.md

# to combine multiple directories into one
protodoc --directories=./parse/testdata=service,dirA=service_message \
	--languages="Go,C++,Java,Python" \
	--title=testdata \
	--output=sample.md
```

Note that parser only understands the minimum syntax
of Protocol Buffer (just enough to generate documentation).

For full featured parser, please check out https://github.com/golang/protobuf.

[cistat]: https://travis-ci.com/etcd-io/protodoc
[protodoc-godoc]: https://godoc.org/go.etcd.io/protodoc
