package server

const Version = "v2"

// TODO: The release version (generated from the git tag) will be the raft
// protocol version for now. When things settle down we will fix it like the
// client API above.
const PeerVersion = releaseVersion
