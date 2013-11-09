package dashboard

import (
	"bytes"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/mod/dashboard/resources"
)

func memoryFileServer(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] %s %s [%s]", req.Method, req.URL.Path, req.RemoteAddr)
	upath := req.URL.Path
	if len(upath) == 0 {
		upath = "index.html"
	}

	// TODO: use the new mux to do this work
	dir, file := path.Split(upath)
	if file == "browser" || file == "stats" {
		file = file + ".html"
	}
	upath = path.Join(dir, file)
	b, ok := resources.File("/" + upath)

	if ok == false {
		http.Error(w, upath+": File not found", http.StatusNotFound)
		return
	}

	http.ServeContent(w, req, upath, time.Time{}, bytes.NewReader(b))
	return
}

// DashboardHttpHandler either uses the compiled in virtual filesystem for the
// dashboard assets or if ETCD_DASHBOARD_DIR is set uses that as the source of
// assets.
func HttpHandler() (handler http.Handler) {
	handler = http.HandlerFunc(memoryFileServer)

	// Serve the dashboard from a filesystem if the magic env variable is enabled
	dashDir := os.Getenv("ETCD_DASHBOARD_DIR")
	if len(dashDir) != 0 {
		log.Debugf("Using dashboard directory %s", dashDir)
		handler = http.FileServer(http.Dir(dashDir))
	}

	return handler
}
