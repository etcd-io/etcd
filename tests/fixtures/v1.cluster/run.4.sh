#!/bin/sh

curl -L http://127.0.0.1:4001/v1/keys/message -d value="Hello world"
curl -L http://127.0.0.1:4001/v1/keys/message -d value="Hello etcd"
curl -L http://127.0.0.1:4001/v1/keys/message -X DELETE
curl -L http://127.0.0.1:4001/v1/keys/message2 -d value="Hola"
curl -L http://127.0.0.1:4001/v1/keys/expiring -d value=bar -d ttl=5
curl -L http://127.0.0.1:4001/v1/keys/foo -d value=one
curl -L http://127.0.0.1:4001/v1/keys/foo -d prevValue=two -d value=three
curl -L http://127.0.0.1:4001/v1/keys/foo -d prevValue=one -d value=two
curl -L http://127.0.0.1:4001/v1/keys/bar -d prevValue= -d value=four
curl -L http://127.0.0.1:4001/v1/keys/bar -d prevValue= -d value=five
curl -X DELETE http://127.0.0.1:7001/remove/node2
