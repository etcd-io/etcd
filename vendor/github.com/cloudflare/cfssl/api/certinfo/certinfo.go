// Package certinfo implements the HTTP handler for the certinfo command.
package certinfo

import (
	"errors"
	"net/http"

	"github.com/cloudflare/cfssl/api"
	"github.com/cloudflare/cfssl/certdb"
	"github.com/cloudflare/cfssl/certinfo"
	"github.com/cloudflare/cfssl/log"
)

// Handler accepts requests for either remote or uploaded
// certificates to be bundled, and returns a certificate bundle (or
// error).
type Handler struct {
	dbAccessor certdb.Accessor
}

// NewHandler creates a new bundler that uses the root bundle and
// intermediate bundle in the trust chain.
func NewHandler() http.Handler {
	return api.HTTPHandler{Handler: new(Handler), Methods: []string{"POST"}}
}

// NewAccessorHandler creates a new bundler with database access via the
// certdb.Accessor interface. If this handler is constructed it will be possible
// to lookup certificates issued earlier by the CA.
func NewAccessorHandler(dbAccessor certdb.Accessor) http.Handler {
	return api.HTTPHandler{
		Handler: &Handler{
			dbAccessor: dbAccessor,
		},
		Methods: []string{"POST"},
	}
}

// Handle implements an http.Handler interface for the bundle handler.
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) (err error) {
	blob, matched, err := api.ProcessRequestFirstMatchOf(r,
		[][]string{
			{"certificate"},
			{"domain"},
			{"serial", "authority_key_id"},
		})
	if err != nil {
		log.Warningf("invalid request: %v", err)
		return err
	}

	var cert *certinfo.Certificate
	switch matched[0] {
	case "domain":
		if cert, err = certinfo.ParseCertificateDomain(blob["domain"]); err != nil {
			log.Warningf("couldn't parse remote certificate: %v", err)
			return err
		}
	case "certificate":
		if cert, err = certinfo.ParseCertificatePEM([]byte(blob["certificate"])); err != nil {
			log.Warningf("bad PEM certificate: %v", err)
			return err
		}
	case "serial", "authority_key_id":
		if h.dbAccessor == nil {
			log.Warning("could not find certificates with db access")

			return errors.New("cannot lookup certificate from serial without db access")
		}

		if cert, err = certinfo.ParseSerialNumber(blob["serial"], blob["authority_key_id"], h.dbAccessor); err != nil {
			log.Warningf("couldn't find certificate: %v", err)

			return err
		}
	}

	return api.SendResponse(w, cert)
}
