$ETCD_PACKAGE="github.com/coreos/etcd"
$env:GOPATH=$pwd.Path
$ETCD_DIR="$env:GOPATH/third_party/src/$ETCD_PACKAGE"
$env:ETCD_DIR=$ETCD_DIR
$env:ETCD_TARGET=$pwd.Path

$ETCD_BASE=(Split-Path $ETCD_DIR -Parent)
#
# for now, get rid of etcd symlink in the repo
if(test-path $ETCD_DIR -pathType leaf){
    del $ETCD_DIR
}
#
# replace etcd symlink with a directory junction
if(-not(test-path $ETCD_DIR)){
	mkdir -force "$ETCD_BASE" > $null
	cmd /c 'mklink /J "%ETCD_DIR%" %ETCD_TARGET%'
}

./scripts/release-version.ps1 | Out-File -Encoding UTF8 server/release_version.go
go run third_party.go install "${ETCD_PACKAGE}"
go run third_party.go install "${ETCD_PACKAGE}/bench"
