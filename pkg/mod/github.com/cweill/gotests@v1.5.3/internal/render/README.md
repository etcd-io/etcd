bindata.go must be generated with https://github.com/jteeuwen/go-bindata.

From the gotests root run `$ go generate ./...`.

Or run `$ go-bindata -pkg=bindata -o "internal/render/bindata/bindata.go" internal/render/templates/`.

During development run `$ go-bindata -pkg=bindata -o "internal/render/bindata/bindata.go" -debug internal/render/templates/` instead.