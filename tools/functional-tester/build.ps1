$pwd = $pwd -replace "\\", "/"
$Build_Command = $SCRIPT:MyInvocation.MyCommand.Path -replace "\\", "/"
$Build_Command = $Build_Command.Replace("$pwd", "")
if ("$Build_Command" -ne "/tools/functional-tester/build.ps1"){
	echo "must be run from repository root"
	exit 255
}
$env:CGO_ENABLED = 0
$env:GO15VENDOREXPERIMENT = 1
$GIT_SHA="$(git rev-parse --short HEAD)"
go build -a -installsuffix cgo -ldflags "-s" -o bin/etcd-agent.exe ./cmd/tools/functional-tester/etcd-agent
go build -a -installsuffix cgo -ldflags "-s" -o bin/etcd-tester.exe ./cmd/tools/functional-tester/etcd-tester
