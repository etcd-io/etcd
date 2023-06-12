// ../../tests/integration/clientv3/examples/example_test.go

package clientv3_test

import (
	"context"
	"log"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/v3"
)

func mockConfig_insecure() {}

func ExampleConfig_insecure() {
	forUnitTestsRunInMockedContext(mockConfig_insecure, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close() // make sure to close the client

		_, err = cli.Put(context.TODO(), "foo", "bar")
		if err != nil {
			log.Fatal(err)
		}
	})

	// Without the line below the test is not being executed

	// Output:
}

func mockConfig_withTLS() {}

func ExampleConfig_withTLS() {
	forUnitTestsRunInMockedContext(mockConfig_withTLS, func() {
		tlsInfo := transport.TLSInfo{
			CertFile:      "/tmp/test-certs/test-name-1.pem",
			KeyFile:       "/tmp/test-certs/test-name-1-key.pem",
			TrustedCAFile: "/tmp/test-certs/trusted-ca.pem",
		}
		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			log.Fatal(err)
		}
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
			TLS:         tlsConfig,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close() // make sure to close the client

		_, err = cli.Put(context.TODO(), "foo", "bar")
		if err != nil {
			log.Fatal(err)
		}
	})
	// Without the line below the test is not being executed
	// Output:
}
