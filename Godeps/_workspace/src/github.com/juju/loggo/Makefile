default: check

check:
	go test && go test -compiler gccgo

docs:
	godoc2md github.com/juju/loggo > README.md
	sed -i 's|\[godoc-link-here\]|[![GoDoc](https://godoc.org/github.com/juju/loggo?status.svg)](https://godoc.org/github.com/juju/loggo)|' README.md 


.PHONY: default check docs
