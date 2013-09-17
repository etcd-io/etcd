
$VER=(git describe --tags HEAD)

@"
package main
const releaseVersion = "$VER"
"@