
$ETCD_PACKAGE="github.com/coreos/etcd"
$env:GOPATH=$pwd.Path
$SRC_DIR="$env:GOPATH/src"
$ETCD_DIR="$SRC_DIR/$ETCD_PACKAGE"
$env:ETCD_DIR="$SRC_DIR/$ETCD_PACKAGE"
$env:ETCD_TARGET=$pwd.Path

$ETCD_BASE=(Split-Path $ETCD_DIR -Parent)
if(-not(test-path $ETCD_DIR)){
	mkdir -force "$ETCD_BASE" > $null
}

if(-not(test-path $ETCD_DIR )){
	cmd /c 'mklink /J "%ETCD_DIR%" %ETCD_TARGET%'
}

foreach($i in (ls third_party/*)){
	if("$i" -eq "third_party/src") {continue}
	
	cp -Recurse -force "$i" src/
}

./scripts/release-version.ps1 | Out-File -Encoding UTF8 server/release_version.go
go build -v "${ETCD_PACKAGE}"
