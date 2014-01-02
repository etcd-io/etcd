
$VER=(git describe --tags HEAD)

@"
package server
const ReleaseVersion = "$VER"
"@