set positional-arguments := true
set shell := ["bash", "-c"]

version := `git describe --tags --dirty --always`
export GOOS := env("GOOS", "linux")
export GOARCH := env("GOARCH", "amd64")

_help:
    @just -l

# Generate README.md from lexer definitions
readme:
    #!/usr/bin/env bash
    GOOS= GOARCH= ./table.py

# Generate tokentype_string.go
tokentype-string:
    go generate

# Format JavaScript files
format-js:
    biome format --write cmd/chromad/static/index.js cmd/chromad/static/chroma.js

# Build chromad binary
chromad: wasm-exec chroma-wasm
    #!/usr/bin/env bash
    rm -rf build
    mk cmd/chromad/static/index.min.js : cmd/chromad/static/{index,chroma}.js -- \
    	esbuild --platform=browser --format=esm --bundle cmd/chromad/static/index.js --minify --external:./wasm_exec.js --outfile=cmd/chromad/static/index.min.js
    mk cmd/chromad/static/index.min.css : cmd/chromad/static/index.css -- \
    	esbuild --bundle cmd/chromad/static/index.css --minify --outfile=cmd/chromad/static/index.min.css
    cd cmd/chromad && CGOENABLED=0 go build -ldflags="-X 'main.version={{ version }}'" -o ../../build/chromad .

# Copy wasm_exec.js from TinyGo
wasm-exec:
    #!/usr/bin/env bash
    tinygoroot=$(tinygo env TINYGOROOT)
    mk cmd/chromad/static/wasm_exec.js : "$tinygoroot/targets/wasm_exec.js" -- \
    	install -m644 "$tinygoroot/targets/wasm_exec.js" cmd/chromad/static/wasm_exec.js

# Build WASM binary
chroma-wasm:
    #!/usr/bin/env bash
    if type tinygo > /dev/null 2>&1; then
        mk cmd/chromad/static/chroma.wasm : cmd/libchromawasm/main.go -- \
        	tinygo build -no-debug -target wasm -o cmd/chromad/static/chroma.wasm cmd/libchromawasm/main.go
    else
        mk cmd/chromad/static/chroma.wasm : cmd/libchromawasm/main.go -- \
        	GOOS=js GOARCH=wasm go build -o cmd/chromad/static/chroma.wasm cmd/libchromawasm/main.go
    fi

# Upload chromad to server
upload: chromad
    scp build/chromad root@swapoff.org:
    ssh root@swapoff.org 'install -m755 ./chromad /srv/http/swapoff.org/bin && service chromad restart'
