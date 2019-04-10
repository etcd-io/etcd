package handlers

import (
	"fmt"
	"net/http"
)

func RobotsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "User-agent: *\nDisallow: /")
}
