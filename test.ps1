# $PKG = @("./store", "./server", "./server/v2/tests", "./mod/lock/v2/tests", "./tests/functional")
$PKG = @("./tests/functional")

# use right GOPATH
$env:GOPATH=$pwd.Path
$env:ETCD_BIN_PATH = "$pwd\bin\etcd.exe"

# Unit tests
foreach ($i in $PKG) {
  go run third_party.go test -i $i
  go run third_party.go test -v $i
}
