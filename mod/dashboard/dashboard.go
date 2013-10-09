package dashboard

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/coreos/etcd/mod/dashboard/resources"
)

func memoryFileServer(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if len(path) == 0 {
		path = "index.html"
	}

	b, ok := resources.File("/" + path)

	if ok == false {
		http.Error(w, path+": File not found", http.StatusNotFound)
		return
	}

	http.ServeContent(w, req, path, time.Time{}, bytes.NewReader(b))
	return
}

// DashboardHttpHandler either uses the compiled in virtual filesystem for the
// dashboard assets or if ETCD_DASHBOARD_DIR is set uses that as the source of
// assets.
func HttpHandler() (handler http.Handler) {
	fmt.Println("hello world")
	handler = http.HandlerFunc(memoryFileServer)

	// Serve the dashboard from a filesystem if the magic env variable is enabled
	dashDir := os.Getenv("ETCD_DASHBOARD_DIR")
	if len(dashDir) != 0 {
		handler = http.FileServer(http.Dir(dashDir))
	}

	return handler
}
