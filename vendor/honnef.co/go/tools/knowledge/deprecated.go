package knowledge

const (
	// DeprecatedNeverUse indicates that an API should never be used, regardless of Go version.
	DeprecatedNeverUse = "never"
	// DeprecatedUseNoLonger indicates that an API has no use anymore.
	DeprecatedUseNoLonger = "no longer"
)

// Deprecation describes when a Go API has been deprecated.
type Deprecation struct {
	// The minor Go version since which this API has been deprecated.
	DeprecatedSince string
	// The minor Go version since which an alternative API has been available.
	// May also be one of DeprecatedNeverUse or DeprecatedUseNoLonger.
	AlternativeAvailableSince string
}

// go/importer.ForCompiler contains "Deprecated:", but it refers to a single argument, not the whole function.
// Luckily, the notice starts in the middle of a paragraph, and as such isn't detected by us.

// TODO(dh): StdlibDeprecations doesn't contain entries for internal packages and unexported API. That's fine for normal
// users, but makes the Deprecated check less useful for people working on Go itself.

// StdlibDeprecations contains a mapping of Go API (such as variables, methods, or fields, among others)
// to information about when it has been deprecated.
var StdlibDeprecations = map[string]Deprecation{
	// FIXME(dh): AllowBinary isn't being detected as deprecated
	// because the comment has a newline right after "Deprecated:"
	"go/build.AllowBinary":                      {"go1.7", "go1.7"},
	"(archive/zip.FileHeader).CompressedSize":   {"go1.1", "go1.1"},
	"(archive/zip.FileHeader).UncompressedSize": {"go1.1", "go1.1"},
	"(archive/zip.FileHeader).ModifiedTime":     {"go1.10", "go1.10"},
	"(archive/zip.FileHeader).ModifiedDate":     {"go1.10", "go1.10"},
	"(*archive/zip.FileHeader).ModTime":         {"go1.10", "go1.10"},
	"(*archive/zip.FileHeader).SetModTime":      {"go1.10", "go1.10"},
	"(go/doc.Package).Bugs":                     {"go1.1", "go1.1"},
	"os.SEEK_SET":                               {"go1.7", "go1.7"},
	"os.SEEK_CUR":                               {"go1.7", "go1.7"},
	"os.SEEK_END":                               {"go1.7", "go1.7"},
	"(net.Dialer).Cancel":                       {"go1.7", "go1.7"},
	"runtime.CPUProfile":                        {"go1.9", "go1.0"},
	"compress/flate.ReadError":                  {"go1.6", DeprecatedUseNoLonger},
	"compress/flate.WriteError":                 {"go1.6", DeprecatedUseNoLonger},
	"path/filepath.HasPrefix":                   {"go1.0", DeprecatedNeverUse},
	"(net/http.Transport).Dial":                 {"go1.7", "go1.7"},
	"(net/http.Transport).DialTLS":              {"go1.14", "go1.14"},
	"(*net/http.Transport).CancelRequest":       {"go1.6", "go1.5"},
	"net/http.ErrWriteAfterFlush":               {"go1.7", DeprecatedUseNoLonger},
	"net/http.ErrHeaderTooLong":                 {"go1.8", DeprecatedUseNoLonger},
	"net/http.ErrShortBody":                     {"go1.8", DeprecatedUseNoLonger},
	"net/http.ErrMissingContentLength":          {"go1.8", DeprecatedUseNoLonger},
	"net/http/httputil.ErrPersistEOF":           {"go1.0", DeprecatedUseNoLonger},
	"net/http/httputil.ErrClosed":               {"go1.0", DeprecatedUseNoLonger},
	"net/http/httputil.ErrPipeline":             {"go1.0", DeprecatedUseNoLonger},
	"net/http/httputil.ServerConn":              {"go1.0", "go1.0"},
	"net/http/httputil.NewServerConn":           {"go1.0", "go1.0"},
	"net/http/httputil.ClientConn":              {"go1.0", "go1.0"},
	"net/http/httputil.NewClientConn":           {"go1.0", "go1.0"},
	"net/http/httputil.NewProxyClientConn":      {"go1.0", "go1.0"},
	"(net/http.Request).Cancel":                 {"go1.7", "go1.7"},
	"(text/template/parse.PipeNode).Line":       {"go1.1", DeprecatedUseNoLonger},
	"(text/template/parse.ActionNode).Line":     {"go1.1", DeprecatedUseNoLonger},
	"(text/template/parse.BranchNode).Line":     {"go1.1", DeprecatedUseNoLonger},
	"(text/template/parse.TemplateNode).Line":   {"go1.1", DeprecatedUseNoLonger},
	"database/sql/driver.ColumnConverter":       {"go1.9", "go1.9"},
	"database/sql/driver.Execer":                {"go1.8", "go1.8"},
	"database/sql/driver.Queryer":               {"go1.8", "go1.8"},
	"(database/sql/driver.Conn).Begin":          {"go1.8", "go1.8"},
	"(database/sql/driver.Stmt).Exec":           {"go1.8", "go1.8"},
	"(database/sql/driver.Stmt).Query":          {"go1.8", "go1.8"},
	"syscall.StringByteSlice":                   {"go1.1", "go1.1"},
	"syscall.StringBytePtr":                     {"go1.1", "go1.1"},
	"syscall.StringSlicePtr":                    {"go1.1", "go1.1"},
	"syscall.StringToUTF16":                     {"go1.1", "go1.1"},
	"syscall.StringToUTF16Ptr":                  {"go1.1", "go1.1"},
	"(*regexp.Regexp).Copy":                     {"go1.12", DeprecatedUseNoLonger},
	"(archive/tar.Header).Xattrs":               {"go1.10", "go1.10"},
	"archive/tar.TypeRegA":                      {"go1.11", "go1.1"},
	"go/types.NewInterface":                     {"go1.11", "go1.11"},
	"(*go/types.Interface).Embedded":            {"go1.11", "go1.11"},
	"go/importer.For":                           {"go1.12", "go1.12"},
	"encoding/json.InvalidUTF8Error":            {"go1.2", DeprecatedUseNoLonger},
	"encoding/json.UnmarshalFieldError":         {"go1.2", DeprecatedUseNoLonger},
	"encoding/csv.ErrTrailingComma":             {"go1.2", DeprecatedUseNoLonger},
	"(encoding/csv.Reader).TrailingComma":       {"go1.2", DeprecatedUseNoLonger},
	"(net.Dialer).DualStack":                    {"go1.12", "go1.12"},
	"net/http.ErrUnexpectedTrailer":             {"go1.12", DeprecatedUseNoLonger},
	"net/http.CloseNotifier":                    {"go1.11", "go1.7"},
	// This is hairy. The notice says "Not all errors in the http package related to protocol errors are of type ProtocolError", but doesn't that imply that some errors do?
	"net/http.ProtocolError":                       {"go1.8", DeprecatedUseNoLonger},
	"(crypto/x509.CertificateRequest).Attributes":  {"go1.5", "go1.3"},
	"(*crypto/x509.Certificate).CheckCRLSignature": {"go1.19", "go1.19"},
	"crypto/x509.ParseCRL":                         {"go1.19", "go1.19"},
	"crypto/x509.ParseDERCRL":                      {"go1.19", "go1.19"},
	"(*crypto/x509.Certificate).CreateCRL":         {"go1.19", "go1.19"},
	"crypto/x509/pkix.TBSCertificateList":          {"go1.19", "go1.19"},
	"crypto/x509/pkix.RevokedCertificate":          {"go1.19", "go1.19"},
	"go/doc.ToHTML":                                {"go1.20", "go1.20"},
	"go/doc.ToText":                                {"go1.20", "go1.20"},
	"go/doc.Synopsis":                              {"go1.20", "go1.20"},
	"math/rand.Seed":                               {"go1.20", "go1.0"},
	"math/rand.Read":                               {"go1.20", DeprecatedNeverUse},

	// These functions have no direct alternative, but they are insecure and should no longer be used.
	"crypto/x509.IsEncryptedPEMBlock": {"go1.16", DeprecatedNeverUse},
	"crypto/x509.DecryptPEMBlock":     {"go1.16", DeprecatedNeverUse},
	"crypto/x509.EncryptPEMBlock":     {"go1.16", DeprecatedNeverUse},
	"crypto/dsa":                      {"go1.16", DeprecatedNeverUse},

	// This function has no alternative, but also no purpose.
	"(*crypto/rc4.Cipher).Reset":                     {"go1.12", DeprecatedNeverUse},
	"(net/http/httptest.ResponseRecorder).HeaderMap": {"go1.11", "go1.7"},
	"image.ZP":                                    {"go1.13", "go1.0"},
	"image.ZR":                                    {"go1.13", "go1.0"},
	"(*debug/gosym.LineTable).LineToPC":           {"go1.2", "go1.2"},
	"(*debug/gosym.LineTable).PCToLine":           {"go1.2", "go1.2"},
	"crypto/tls.VersionSSL30":                     {"go1.13", DeprecatedNeverUse},
	"(crypto/tls.Config).NameToCertificate":       {"go1.14", DeprecatedUseNoLonger},
	"(*crypto/tls.Config).BuildNameToCertificate": {"go1.14", DeprecatedUseNoLonger},
	"(crypto/tls.Config).SessionTicketKey":        {"go1.16", "go1.5"},
	// No alternative, no use
	"(crypto/tls.ConnectionState).NegotiatedProtocolIsMutual": {"go1.16", DeprecatedNeverUse},
	// No alternative, but insecure
	"(crypto/tls.ConnectionState).TLSUnique": {"go1.16", DeprecatedNeverUse},
	"image/jpeg.Reader":                      {"go1.4", DeprecatedNeverUse},

	// All of these have been deprecated in favour of external libraries
	"syscall.AttachLsf":                     {"go1.7", "go1.0"},
	"syscall.DetachLsf":                     {"go1.7", "go1.0"},
	"syscall.LsfSocket":                     {"go1.7", "go1.0"},
	"syscall.SetLsfPromisc":                 {"go1.7", "go1.0"},
	"syscall.LsfJump":                       {"go1.7", "go1.0"},
	"syscall.LsfStmt":                       {"go1.7", "go1.0"},
	"syscall.BpfStmt":                       {"go1.7", "go1.0"},
	"syscall.BpfJump":                       {"go1.7", "go1.0"},
	"syscall.BpfBuflen":                     {"go1.7", "go1.0"},
	"syscall.SetBpfBuflen":                  {"go1.7", "go1.0"},
	"syscall.BpfDatalink":                   {"go1.7", "go1.0"},
	"syscall.SetBpfDatalink":                {"go1.7", "go1.0"},
	"syscall.SetBpfPromisc":                 {"go1.7", "go1.0"},
	"syscall.FlushBpf":                      {"go1.7", "go1.0"},
	"syscall.BpfInterface":                  {"go1.7", "go1.0"},
	"syscall.SetBpfInterface":               {"go1.7", "go1.0"},
	"syscall.BpfTimeout":                    {"go1.7", "go1.0"},
	"syscall.SetBpfTimeout":                 {"go1.7", "go1.0"},
	"syscall.BpfStats":                      {"go1.7", "go1.0"},
	"syscall.SetBpfImmediate":               {"go1.7", "go1.0"},
	"syscall.SetBpf":                        {"go1.7", "go1.0"},
	"syscall.CheckBpfVersion":               {"go1.7", "go1.0"},
	"syscall.BpfHeadercmpl":                 {"go1.7", "go1.0"},
	"syscall.SetBpfHeadercmpl":              {"go1.7", "go1.0"},
	"syscall.RouteRIB":                      {"go1.8", "go1.0"},
	"syscall.RoutingMessage":                {"go1.8", "go1.0"},
	"syscall.RouteMessage":                  {"go1.8", "go1.0"},
	"syscall.InterfaceMessage":              {"go1.8", "go1.0"},
	"syscall.InterfaceAddrMessage":          {"go1.8", "go1.0"},
	"syscall.ParseRoutingMessage":           {"go1.8", "go1.0"},
	"syscall.ParseRoutingSockaddr":          {"go1.8", "go1.0"},
	"syscall.InterfaceAnnounceMessage":      {"go1.7", "go1.0"},
	"syscall.InterfaceMulticastAddrMessage": {"go1.7", "go1.0"},
	"syscall.FormatMessage":                 {"go1.5", "go1.0"},
	"syscall.PostQueuedCompletionStatus":    {"go1.17", "go1.0"},
	"syscall.GetQueuedCompletionStatus":     {"go1.17", "go1.0"},
	"syscall.CreateIoCompletionPort":        {"go1.17", "go1.0"},

	// We choose to only track the package itself, even though all functions are deprecated individually, too. Anyone
	// using ioutil directly will have to import it, and this keeps the noise down.
	"io/ioutil": {"go1.19", "go1.19"},

	"bytes.Title":   {"go1.18", "go1.0"},
	"strings.Title": {"go1.18", "go1.0"},
	"(crypto/tls.Config).PreferServerCipherSuites": {"go1.18", DeprecatedUseNoLonger},
	// It's not clear if Subjects was okay to use in the past, so we err on the less noisy side of assuming that it was.
	"(*crypto/x509.CertPool).Subjects": {"go1.18", DeprecatedUseNoLonger},
	"go/types.NewSignature":            {"go1.18", "go1.18"},
	"(net.Error).Temporary":            {"go1.18", DeprecatedNeverUse},
	// InterfaceData is another tricky case. It was deprecated in Go 1.18, but has been useless since Go 1.4, and an
	// "alternative" (using your own unsafe hacks) has existed forever. We don't want to get into hairsplitting with
	// users who somehow successfully used this between 1.4 and 1.18, so we'll just tag it as deprecated since 1.18.
	"(reflect.Value).InterfaceData": {"go1.18", "go1.18"},

	// The following objects are only deprecated on Windows.
	"syscall.Syscall":   {"go1.18", "go1.18"},
	"syscall.Syscall12": {"go1.18", "go1.18"},
	"syscall.Syscall15": {"go1.18", "go1.18"},
	"syscall.Syscall18": {"go1.18", "go1.18"},
	"syscall.Syscall6":  {"go1.18", "go1.18"},
	"syscall.Syscall9":  {"go1.18", "go1.18"},

	"reflect.SliceHeader":                           {"go1.21", "go1.17"},
	"reflect.StringHeader":                          {"go1.21", "go1.20"},
	"crypto/elliptic.GenerateKey":                   {"go1.21", "go1.21"},
	"crypto/elliptic.Marshal":                       {"go1.21", "go1.21"},
	"crypto/elliptic.Unmarshal":                     {"go1.21", "go1.21"},
	"(*crypto/elliptic.CurveParams).Add":            {"go1.21", "go1.21"},
	"(*crypto/elliptic.CurveParams).Double":         {"go1.21", "go1.21"},
	"(*crypto/elliptic.CurveParams).IsOnCurve":      {"go1.21", "go1.21"},
	"(*crypto/elliptic.CurveParams).ScalarBaseMult": {"go1.21", "go1.21"},
	"(*crypto/elliptic.CurveParams).ScalarMult":     {"go1.21", "go1.21"},
	"(crypto/elliptic.Curve).Add":                   {"go1.21", "go1.21"},
	"(crypto/elliptic.Curve).Double":                {"go1.21", "go1.21"},
	"(crypto/elliptic.Curve).IsOnCurve":             {"go1.21", "go1.21"},
	"(crypto/elliptic.Curve).ScalarBaseMult":        {"go1.21", "go1.21"},
	"(crypto/elliptic.Curve).ScalarMult":            {"go1.21", "go1.21"},

	"crypto/rsa.GenerateMultiPrimeKey":                 {"go1.21", DeprecatedNeverUse},
	"(crypto/rsa.PrecomputedValues).CRTValues":         {"go1.21", DeprecatedNeverUse},
	"(crypto/x509.RevocationList).RevokedCertificates": {"go1.21", "go1.21"},

	"go/ast.NewPackage":           {"go1.22", "go1.0"},
	"go/ast.Importer":             {"go1.22", "go1.0"},
	"go/ast.Object":               {"go1.22", "go1.0"},
	"go/ast.Package":              {"go1.22", "go1.0"},
	"go/ast.Scope":                {"go1.22", "go1.0"},
	"html/template.ErrJSTemplate": {"go1.22", DeprecatedUseNoLonger},
	"reflect.PtrTo":               {"go1.22", "go1.18"},

	// Technically, runtime.GOROOT could be considered DeprecatedNeverUse, but
	// using it used to be a lot more common and accepted.
	"runtime.GOROOT": {"go1.24", DeprecatedUseNoLonger},
	// These are never safe to use; a concrete alternative was added in Go 1.2 (crypto/cipher.AEAD).
	"crypto/cipher.NewCFBDecrypter": {"go1.24", "go1.2"},
	"crypto/cipher.NewCFBEncrypter": {"go1.24", "go1.2"},
	"crypto/cipher.NewOFB":          {"go1.24", "go1.2"},

	"go/ast.FilterFuncDuplicates":       {"go1.25", "go1.0"},
	"go/ast.FilterImportDuplicates":     {"go1.25", "go1.0"},
	"go/ast.FilterUnassociatedComments": {"go1.25", "go1.0"},
	"go/ast.FilterPackage":              {"go1.25", "go1.0"},
	"go/ast.MergePackageFiles":          {"go1.25", "go1.0"},
	"go/ast.PackageExports":             {"go1.25", "go1.0"},
	"go/ast.MergeMode":                  {"go1.25", "go1.0"},
	// Go 1.11 because that's around the time x/tools/go/packages was released.
	"go/parser.ParseDir": {"go1.25", "go1.11"},

	// Go 1.25 is the first version to provide all of the alternatives mentioned
	// by the deprecation note.
	"(crypto/ecdsa.PublicKey).X":  {"go1.26", "go1.25"},
	"(crypto/ecdsa.PublicKey).Y":  {"go1.26", "go1.25"},
	"(crypto/ecdsa.PrivateKey).D": {"go1.26", "go1.25"},

	"crypto/rsa.DecryptPKCS1v15":           {"go1.26", DeprecatedNeverUse},
	"crypto/rsa.DecryptPKCS1v15SessionKey": {"go1.26", DeprecatedNeverUse},
	"crypto/rsa.PKCS1v15DecryptOptions":    {"go1.26", DeprecatedNeverUse},
	"crypto/rsa.EncryptPKCS1v15":           {"go1.26", DeprecatedNeverUse},

	"(net/http/httputil.ReverseProxy).Director": {"go1.26", "go1.20"},
}

// Last imported from GOROOT/api/go1.26.txt at d3ddc4854429185e6e06ca1f7628bb790404abb5.
