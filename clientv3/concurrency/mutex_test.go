package concurrency_test

import (
	"context"
	"sync"
	"testing"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
)

func BenchmarkMutex(b *testing.B) {
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		b.Fatal(err)
	}
	defer cli.Close()

	ctx := context.Background()

	var berr error
	var once sync.Once

	b.SetParallelism(64)
	b.RunParallel(func(pb *testing.PB) {
		s, err := concurrency.NewSession(cli)
		if err != nil {

			return
		}
		defer once.Do(func() { berr = s.Close() })

		foo := 0
		for pb.Next() {
			m := concurrency.NewMutex(s, "/bench-lock")

			if err := m.Lock(ctx); err != nil {
				once.Do(func() { berr = err })
				return
			}
			foo += 1
			if err := m.Unlock(ctx); err != nil {
				once.Do(func() { berr = err })
				return
			}
		}
		_ = foo
	})

	if berr != nil {
		b.Fatal(berr)
	}
}

func BenchmarkMutexMulti(b *testing.B) {
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		b.Fatal(err)
	}
	defer cli.Close()

	s, err := concurrency.NewSession(cli)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	var berr error
	var once sync.Once

	b.SetParallelism(64)
	b.RunParallel(func(pb *testing.PB) {
		foo := 0
		for pb.Next() {
			m := concurrency.NewMutex(s, "/bench-lock2")

			if err := m.Lock(ctx); err != nil {
				once.Do(func() { berr = err })
				return
			}
			foo += 1
			if err := m.Unlock(ctx); err != nil {
				once.Do(func() { berr = err })
				return
			}
		}
		_ = foo
	})

	if berr != nil {
		b.Fatal(berr)
	}
}
