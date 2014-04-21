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

	b, err := resources.Asset(req.URL.Path)

	if err != nil {
		http.Error(w, upath+": File not found", http.StatusNotFound)
		return
	}

	http.ServeContent(w, req, upath, time.Time{}, bytes.NewReader(b))
	return
}

func getDashDir() string {
	return os.Getenv("ETCD_DASHBOARD_DIR")
}

// DashboardHttpHandler either uses the compiled in virtual filesystem for the
// dashboard assets or if ETCD_DASHBOARD_DIR is set uses that as the source of
// assets.
func HttpHandler() (handler http.Handler) {
	handler = http.HandlerFunc(memoryFileServer)

	// Serve the dashboard from a filesystem if the magic env variable is enabled
	dashDir := getDashDir()
	if len(dashDir) != 0 {
		log.Debugf("Using dashboard directory %s", dashDir)
		handler = http.FileServer(http.Dir(dashDir))
	}

	return handler
}

// Always returns the index.html page.
func IndexPage(w http.ResponseWriter, req *http.Request) {
	dashDir := getDashDir()
	if len(dashDir) != 0 {
		// Serve the index page from disk if the env variable is set.
		http.ServeFile(w, req, path.Join(dashDir, "index.html"))
	} else {
		// Otherwise serve it from the compiled resources.
		b, _ := resources.Asset("index.html")
		http.ServeContent(w, req, "index.html", time.Time{}, bytes.NewReader(b))
	}
	return
}
