# get-usable-mod-version-from-tag.sh

Tags like `v3.X.Y` do not work with `go.mod` files.
This tiny script will compute a usable go module version to import from a tag (until the situation gets fixed).

## Usage
Get the usable version from a tag:
```
get-usable-mod-version-from-tag.sh -t tag_name
```
You can now use it in `go.mod` (the version here is for tag `v3.4.3`).
```
require go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738
```
## go get

If you're using go get, fetch the package with:
```
go get go.etcd.io/etcd@v0.0.0-20191023171146-3cf2f69b5738
```
And when it's done, add this line at the end of go.mod:
```
replace go.etcd.io/etcd => go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738
```
