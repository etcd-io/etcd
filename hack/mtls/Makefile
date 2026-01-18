.PHONY: all ca1 ca2 certs-ca1 certs-ca2-node2 certs-ca2-node3 certs-ca2-node4 certs-ca2-client clean start test-rotation

CFSSL = cfssl
JSON = cfssljson

all: ca1 certs-ca1

ca1:
	mkdir -p certs
	$(CFSSL) gencert -initca config/ca-csr.json | $(JSON) -bare certs/ca1

ca2:
	mkdir -p certs
	$(CFSSL) gencert -initca config/ca-csr.json | $(JSON) -bare certs/ca2

# Generate certs for initial 3-node cluster from CA1
certs-ca1: ca1
	$(CFSSL) gencert -ca certs/ca1.pem -ca-key certs/ca1-key.pem -config config/ca-config.json config/req-csr.json | $(JSON) -bare certs/etcd1
	$(CFSSL) gencert -ca certs/ca1.pem -ca-key certs/ca1-key.pem -config config/ca-config.json config/req-csr.json | $(JSON) -bare certs/etcd2
	$(CFSSL) gencert -ca certs/ca1.pem -ca-key certs/ca1-key.pem -config config/ca-config.json config/req-csr.json | $(JSON) -bare certs/etcd3
	$(CFSSL) gencert -ca certs/ca1.pem -ca-key certs/ca1-key.pem -config config/ca-config.json config/req-csr.json | $(JSON) -bare certs/client
	cp certs/ca1.pem certs/ca.pem

# Generate CA2 certs for rotation
certs-ca2-node2:
	$(CFSSL) gencert -ca certs/ca2.pem -ca-key certs/ca2-key.pem -config config/ca-config.json config/req-csr.json | $(JSON) -bare certs/etcd2

certs-ca2-node3:
	$(CFSSL) gencert -ca certs/ca2.pem -ca-key certs/ca2-key.pem -config config/ca-config.json config/req-csr.json | $(JSON) -bare certs/etcd3

certs-ca2-node4:
	$(CFSSL) gencert -ca certs/ca2.pem -ca-key certs/ca2-key.pem -config config/ca-config.json config/req-csr.json | $(JSON) -bare certs/etcd4

certs-ca2-client:
	$(CFSSL) gencert -ca certs/ca2.pem -ca-key certs/ca2-key.pem -config config/ca-config.json config/req-csr.json | $(JSON) -bare certs/client

clean:
	rm -rf certs *.etcd

start: all
	goreman -f Procfile start

test-rotation:
	./test-rotation.sh
