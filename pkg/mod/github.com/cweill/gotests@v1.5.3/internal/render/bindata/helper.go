// Package bindata
// Helper with wrapper for backward compatibility with go-bindata
// used only AssetNames func, because only this func has no analog on esc
//
// the reason for changing go-bindata to esc - is:
// `go-bindata creator deleted their @github account and someone else created a new account with the same name.`
//
// https://github.com/jteeuwen/go-bindata/issues/5
// https://twitter.com/francesc/status/961249107020001280
//
// After research some of alternatives - `esc` - is looks like one of the best choice
// https://tech.townsourced.com/post/embedding-static-files-in-go/

package bindata

// AssetNames returns the names of the assets. (for compatible with go-bindata)
func AssetNames() []string {
	names := make([]string, 0, len(_escData))
	for name := range _escData {
		names = append(names, name)
	}
	return names
}
