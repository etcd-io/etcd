$PKG = @("./store", "./server", "./server/v2/tests", "./mod/lock/v2/tests")

# use right GOPATH
$env:GOPATH=$pwd.Path

# Unit tests
foreach ($i in $PKG) {
  go test -i $i
  go test -v $i
}

# Functional tests
go test -i ./tests/functional
go test -v ./tests/functional
