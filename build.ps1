$ORG_PATH="github.com/coreos"
$REPO_PATH="$ORG_PATH/etcd"
$PWD = $((Get-Item -Path ".\" -Verbose).FullName)
$GO_LDFLAGS="-s"

echo "Building etcd"

# Set $Env:GO_LDFLAGS=" "(space) for building with all symbols for debugging.
if ($Env:GO_LDFLAGS.length -gt 0) {
	$GO_LDFLAGS=$Env:GO_LDFLAGS
}
$GO_LDFLAGS="$GO_LDFLAGS -X $REPO_PATH/cmd/vendor/$REPO_PATH/version.GitSHA=$GIT_SHA"

# rebuild symlinks
echo ""
echo "Re-creating symlinks"
git ls-files -s cmd | select-string -pattern 120000 | ForEach {
	echo ""	
	$l = $_.ToString()
	$lnkname = $l.Split('	')[1]
	$target = "$(git log -p HEAD -- $lnkname | select -last 2 | select -first 1)"
	$target = $target.SubString(1,$target.Length-1).Replace("/","\")
	$lnkname = $lnkname.Replace("/","\")

	$terms = $lnkname.Split("\")
	$dirname = $terms[0..($terms.length-2)] -join "\"
	$lnkname = "$PWD\$lnkname"
	$targetAbs = "$((Get-Item -Path "$dirname\$target").FullName)"
	$targetAbs = $targetAbs.Replace("/", "\")

	if (test-path -pathtype container "$targetAbs") {
		if (Test-Path "$lnkname") {
			if ((Get-Item "$lnkname") -is [System.IO.DirectoryInfo]) {
				# rd so deleting junction doesn't take files with it
				echo  "rd `"$lnkname`""
				cmd /c rd  "$lnkname"
			}
		}
		if (Test-Path "$lnkname") {
			if (!((Get-Item "$lnkname") -is [System.IO.DirectoryInfo])) {
				echo  "del /A /F `"$lnkname`""
				cmd /c del /A /F  "$lnkname"
			}
		}
		echo  "mklink /J `"$lnkname`" `"$targetAbs`""
		cmd /c mklink /J  "$lnkname"   "$targetAbs"
	} else {
		# Remove file with symlink data (first run)
		if (Test-Path "$lnkname") {
			echo  "del /A /F `"$lnkname`""
			cmd /c del /A /F  "$lnkname"
		}
		echo  "mklink /H `"$lnkname`" `"$targetAbs`""
		cmd /c mklink /H  "$lnkname"   "$targetAbs"
	}
}

if (-not $env:GOPATH) {
	echo ""
	echo "Setting up GOPATH"
	$orgpath="$PWD\gopath\src\" + $ORG_PATH.Replace("/", "\")
	echo ""
	echo "Remove and create junction to $orgpath\etcd"
	if (Test-Path "$orgpath\etcd") {
		if ((Get-Item "$orgpath\etcd") -is [System.IO.DirectoryInfo]) {
			# rd so deleting junction doesn't take files with it
			echo  "rd `"$orgpath\etcd`""
			cmd /c rd  "$orgpath\etcd"
		}
	}
	if (Test-Path "$orgpath") {
		if ((Get-Item "$orgpath") -is [System.IO.DirectoryInfo]) {
			# rd so deleting junction doesn't take files with it
			echo  "rd `"$orgpath`""
			cmd /c rd  "$orgpath"
		}
	}
	if (Test-Path "$orgpath") {
		if (!((Get-Item "$orgpath") -is [System.IO.DirectoryInfo])) {
			# Remove file with symlink data (first run)
			echo  "del /A /F `"$orgpath`""
			cmd /c del /A /F  "$orgpath"
		}
	}
	echo  "mkdir `"$orgpath`""
	cmd /c mkdir  "$orgpath"
	echo  "mklink /J `"$orgpath\etcd`" `"$PWD`""
	cmd /c mklink /J  "$orgpath\etcd"   "$PWD"
	$env:GOPATH = "$PWD\gopath"
}

echo ""
echo "GOPATH = $env:GOPATH"

# Static compilation is useful when etcd is run in a container
echo ""
echo "Building statically linked artifacts"
$env:CGO_ENABLED = 0
$env:GO15VENDOREXPERIMENT = 1
$GIT_SHA="$(git rev-parse --short HEAD)"
go build -a -installsuffix cgo -ldflags $GO_LDFLAGS -o bin\etcd.exe "$REPO_PATH\cmd\etcd"
go build -a -installsuffix cgo -ldflags $GO_LDFLAGS -o bin\etcdctl.exe "$REPO_PATH\cmd\etcdctl"

echo ""
echo "Build complete"
echo "Build artifacts available at `"$PWD\bin`""
