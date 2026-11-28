# testdata

`corrupt-freelist.db` is the snapshot attached to
https://github.com/etcd-io/etcd/issues/22750 (sha256
`336a17f8a0dab5529ebcc0ad276d9d22683b3e80e5fdbefcdf5c0a84ed1164ea`).
Its freelist is corrupted, so bbolt panics with `invalid page allocation: 0`
while etcd commits the membership trim during `etcdutl snapshot restore`.
