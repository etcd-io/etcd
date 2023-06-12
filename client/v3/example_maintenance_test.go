// ../../tests/integration/clientv3/examples/example_maintenance_test.go

package clientv3_test

import (
	"context"
	"log"

	"go.etcd.io/etcd/client/v3"
)

func mockMaintenance_status() {}

func ExampleMaintenance_status() {
	forUnitTestsRunInMockedContext(mockMaintenance_status, func() {
		for _, ep := range exampleEndpoints() {
			cli, err := clientv3.New(clientv3.Config{
				Endpoints:   []string{ep},
				DialTimeout: dialTimeout,
			})
			if err != nil {
				log.Fatal(err)
			}
			defer cli.Close()

			_, err = cli.Status(context.Background(), ep)
			if err != nil {
				log.Fatal(err)
			}
		}
	})
	// Output:
}

func mockMaintenance_defragment() {}

func ExampleMaintenance_defragment() {
	forUnitTestsRunInMockedContext(mockMaintenance_defragment, func() {
		for _, ep := range exampleEndpoints() {
			cli, err := clientv3.New(clientv3.Config{
				Endpoints:   []string{ep},
				DialTimeout: dialTimeout,
			})
			if err != nil {
				log.Fatal(err)
			}
			defer cli.Close()

			if _, err = cli.Defragment(context.TODO(), ep); err != nil {
				log.Fatal(err)
			}
		}
	})
	// Output:
}
