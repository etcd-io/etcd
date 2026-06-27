package certadd

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/cloudflare/cfssl/api"
	"github.com/cloudflare/cfssl/certdb"
	"github.com/cloudflare/cfssl/errors"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/ocsp"
	"github.com/jmoiron/sqlx/types"

	stdocsp "golang.org/x/crypto/ocsp"
)

// This is patterned on
// https://github.com/cloudflare/cfssl/blob/master/api/revoke/revoke.go. This
// file defines an HTTP endpoint handler that accepts certificates and
// inserts them into a certdb, optionally also creating an OCSP
// response for them. If so, it will also return the OCSP response as
// a base64 encoded string.

// A Handler accepts new SSL certificates and inserts them into the
// certdb, creating an appropriate OCSP response for them.
type Handler struct {
	dbAccessor certdb.Accessor
	signer     ocsp.Signer
}

// NewHandler creates a new Handler from a certdb.Accessor and ocsp.Signer
func NewHandler(dbAccessor certdb.Accessor, signer ocsp.Signer) http.Handler {
	return &api.HTTPHandler{
		Handler: &Handler{
			dbAccessor: dbAccessor,
			signer:     signer,
		},
		Methods: []string{"POST"},
	}
}

// AddRequest describes a request from a client to insert a
// certificate into the database.
type AddRequest struct {
	Serial       string         `json:"serial_number"`
	AKI          string         `json:"authority_key_identifier"`
	CALabel      string         `json:"ca_label"`
	Status       string         `json:"status"`
	Reason       int            `json:"reason"`
	Expiry       time.Time      `json:"expiry"`
	RevokedAt    time.Time      `json:"revoked_at"`
	PEM          string         `json:"pem"`
	IssuedAt     *time.Time     `json:"issued_at"`
	NotBefore    *time.Time     `json:"not_before"`
	MetadataJSON types.JSONText `json:"metadata"`
	SansJSON     types.JSONText `json:"sans"`
	CommonName   string         `json:"common_name"`
}

// Map of valid reason codes
var validReasons = map[int]bool{
	stdocsp.Unspecified:          true,
	stdocsp.KeyCompromise:        true,
	stdocsp.CACompromise:         true,
	stdocsp.AffiliationChanged:   true,
	stdocsp.Superseded:           true,
	stdocsp.CessationOfOperation: true,
	stdocsp.CertificateHold:      true,
	stdocsp.RemoveFromCRL:        true,
	stdocsp.PrivilegeWithdrawn:   true,
	stdocsp.AACompromise:         true,
}

// Handle handles HTTP requests to add certificates
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body.Close()

	var req AddRequest

	err = json.Unmarshal(body, &req)
	if err != nil {
		return errors.NewBadRequestString("Unable to parse certificate addition request")
	}

	if len(req.Serial) == 0 {
		return errors.NewBadRequestString("Serial number is required but not provided")
	}

	if len(req.AKI) == 0 {
		return errors.NewBadRequestString("Authority key identifier is required but not provided")
	}

	if _, present := ocsp.StatusCode[req.Status]; !present {
		return errors.NewBadRequestString("Invalid certificate status")
	}

	if ocsp.StatusCode[req.Status] == stdocsp.Revoked {
		if req.RevokedAt == (time.Time{}) {
			return errors.NewBadRequestString("Revoked certificate should specify when it was revoked")
		}

		if _, present := validReasons[req.Reason]; !present {
			return errors.NewBadRequestString("Invalid certificate status reason code")
		}
	}

	if len(req.PEM) == 0 {
		return errors.NewBadRequestString("The provided certificate is empty")
	}

	if req.Expiry.IsZero() {
		return errors.NewBadRequestString("Expiry is required but not provided")
	}

	// Parse the certificate and validate that it matches
	cert, err := helpers.ParseCertificatePEM([]byte(req.PEM))
	if err != nil {
		return errors.NewBadRequestString("Unable to parse PEM encoded certificates")
	}

	serialBigInt := new(big.Int)
	if _, success := serialBigInt.SetString(req.Serial, 10); !success {
		return errors.NewBadRequestString("Unable to parse serial key of request")
	}

	if serialBigInt.Cmp(cert.SerialNumber) != 0 {
		return errors.NewBadRequestString("Serial key of request and certificate do not match")
	}

	aki, err := hex.DecodeString(req.AKI)
	if err != nil {
		return errors.NewBadRequestString("Unable to decode authority key identifier")
	}

	if !bytes.Equal(aki, cert.AuthorityKeyId) {
		return errors.NewBadRequestString("Authority key identifier of request and certificate do not match")
	}

	if req.Expiry != cert.NotAfter {
		return errors.NewBadRequestString("Expiry of request and certificate do not match")
	}

	cr := certdb.CertificateRecord{
		Serial:       req.Serial,
		AKI:          req.AKI,
		CALabel:      req.CALabel,
		Status:       req.Status,
		Reason:       req.Reason,
		Expiry:       req.Expiry,
		RevokedAt:    req.RevokedAt,
		PEM:          req.PEM,
		IssuedAt:     req.IssuedAt,
		NotBefore:    req.NotBefore,
		MetadataJSON: req.MetadataJSON,
		SANsJSON:     req.SansJSON,
		CommonName:   sql.NullString{String: req.CommonName, Valid: req.CommonName != ""},
	}

	err = h.dbAccessor.InsertCertificate(cr)
	if err != nil {
		return err
	}

	result := map[string]string{}

	if h.signer != nil {
		// Now create an appropriate OCSP response
		sr := ocsp.SignRequest{
			Certificate: cert,
			Status:      req.Status,
			Reason:      req.Reason,
			RevokedAt:   req.RevokedAt,
		}
		ocspResponse, err := h.signer.Sign(sr)
		if err != nil {
			return err
		}

		// We parse the OCSP response in order to get the next
		// update time/expiry time
		ocspParsed, err := stdocsp.ParseResponse(ocspResponse, nil)
		if err != nil {
			return err
		}

		result["ocsp_response"] = base64.StdEncoding.EncodeToString(ocspResponse)

		ocspRecord := certdb.OCSPRecord{
			Serial: req.Serial,
			AKI:    req.AKI,
			Body:   string(ocspResponse),
			Expiry: ocspParsed.NextUpdate,
		}

		if err = h.dbAccessor.InsertOCSP(ocspRecord); err != nil {
			return err
		}
	}

	return api.SendResponse(w, result)
}
