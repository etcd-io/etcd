package mapping

import (
	"crypto"
	"crypto/tls"
	"database/sql"
	"go/constant"
	"net/http"
	"net/rpc"
	"strconv"
	"time"
)

var CryptoHash = map[string]string{
	crypto.MD4.String():         "crypto.MD4.String()",
	crypto.MD5.String():         "crypto.MD5.String()",
	crypto.SHA1.String():        "crypto.SHA1.String()",
	crypto.SHA224.String():      "crypto.SHA224.String()",
	crypto.SHA256.String():      "crypto.SHA256.String()",
	crypto.SHA384.String():      "crypto.SHA384.String()",
	crypto.SHA512.String():      "crypto.SHA512.String()",
	crypto.MD5SHA1.String():     "crypto.MD5SHA1.String()",
	crypto.RIPEMD160.String():   "crypto.RIPEMD160.String()",
	crypto.SHA3_224.String():    "crypto.SHA3_224.String()",
	crypto.SHA3_256.String():    "crypto.SHA3_256.String()",
	crypto.SHA3_384.String():    "crypto.SHA3_384.String()",
	crypto.SHA3_512.String():    "crypto.SHA3_512.String()",
	crypto.SHA512_224.String():  "crypto.SHA512_224.String()",
	crypto.SHA512_256.String():  "crypto.SHA512_256.String()",
	crypto.BLAKE2s_256.String(): "crypto.BLAKE2s_256.String()",
	crypto.BLAKE2b_256.String(): "crypto.BLAKE2b_256.String()",
	crypto.BLAKE2b_384.String(): "crypto.BLAKE2b_384.String()",
	crypto.BLAKE2b_512.String(): "crypto.BLAKE2b_512.String()",
}

var HTTPMethod = map[string]string{
	http.MethodGet:     "http.MethodGet",
	http.MethodHead:    "http.MethodHead",
	http.MethodPost:    "http.MethodPost",
	http.MethodPut:     "http.MethodPut",
	http.MethodPatch:   "http.MethodPatch",
	http.MethodDelete:  "http.MethodDelete",
	http.MethodConnect: "http.MethodConnect",
	http.MethodOptions: "http.MethodOptions",
	http.MethodTrace:   "http.MethodTrace",
}

var HTTPStatusCode = map[string]string{
	strconv.Itoa(http.StatusContinue):           "http.StatusContinue",
	strconv.Itoa(http.StatusSwitchingProtocols): "http.StatusSwitchingProtocols",
	strconv.Itoa(http.StatusProcessing):         "http.StatusProcessing",
	strconv.Itoa(http.StatusEarlyHints):         "http.StatusEarlyHints",

	strconv.Itoa(http.StatusOK):                   "http.StatusOK",
	strconv.Itoa(http.StatusCreated):              "http.StatusCreated",
	strconv.Itoa(http.StatusAccepted):             "http.StatusAccepted",
	strconv.Itoa(http.StatusNonAuthoritativeInfo): "http.StatusNonAuthoritativeInfo",
	strconv.Itoa(http.StatusNoContent):            "http.StatusNoContent",
	strconv.Itoa(http.StatusResetContent):         "http.StatusResetContent",
	strconv.Itoa(http.StatusPartialContent):       "http.StatusPartialContent",
	strconv.Itoa(http.StatusMultiStatus):          "http.StatusMultiStatus",
	strconv.Itoa(http.StatusAlreadyReported):      "http.StatusAlreadyReported",
	strconv.Itoa(http.StatusIMUsed):               "http.StatusIMUsed",

	strconv.Itoa(http.StatusMultipleChoices):   "http.StatusMultipleChoices",
	strconv.Itoa(http.StatusMovedPermanently):  "http.StatusMovedPermanently",
	strconv.Itoa(http.StatusFound):             "http.StatusFound",
	strconv.Itoa(http.StatusSeeOther):          "http.StatusSeeOther",
	strconv.Itoa(http.StatusNotModified):       "http.StatusNotModified",
	strconv.Itoa(http.StatusUseProxy):          "http.StatusUseProxy",
	strconv.Itoa(http.StatusTemporaryRedirect): "http.StatusTemporaryRedirect",
	strconv.Itoa(http.StatusPermanentRedirect): "http.StatusPermanentRedirect",

	strconv.Itoa(http.StatusBadRequest):                   "http.StatusBadRequest",
	strconv.Itoa(http.StatusUnauthorized):                 "http.StatusUnauthorized",
	strconv.Itoa(http.StatusPaymentRequired):              "http.StatusPaymentRequired",
	strconv.Itoa(http.StatusForbidden):                    "http.StatusForbidden",
	strconv.Itoa(http.StatusNotFound):                     "http.StatusNotFound",
	strconv.Itoa(http.StatusMethodNotAllowed):             "http.StatusMethodNotAllowed",
	strconv.Itoa(http.StatusNotAcceptable):                "http.StatusNotAcceptable",
	strconv.Itoa(http.StatusProxyAuthRequired):            "http.StatusProxyAuthRequired",
	strconv.Itoa(http.StatusRequestTimeout):               "http.StatusRequestTimeout",
	strconv.Itoa(http.StatusConflict):                     "http.StatusConflict",
	strconv.Itoa(http.StatusGone):                         "http.StatusGone",
	strconv.Itoa(http.StatusLengthRequired):               "http.StatusLengthRequired",
	strconv.Itoa(http.StatusPreconditionFailed):           "http.StatusPreconditionFailed",
	strconv.Itoa(http.StatusRequestEntityTooLarge):        "http.StatusRequestEntityTooLarge",
	strconv.Itoa(http.StatusRequestURITooLong):            "http.StatusRequestURITooLong",
	strconv.Itoa(http.StatusUnsupportedMediaType):         "http.StatusUnsupportedMediaType",
	strconv.Itoa(http.StatusRequestedRangeNotSatisfiable): "http.StatusRequestedRangeNotSatisfiable",
	strconv.Itoa(http.StatusExpectationFailed):            "http.StatusExpectationFailed",
	strconv.Itoa(http.StatusTeapot):                       "http.StatusTeapot",
	strconv.Itoa(http.StatusMisdirectedRequest):           "http.StatusMisdirectedRequest",
	strconv.Itoa(http.StatusUnprocessableEntity):          "http.StatusUnprocessableEntity",
	strconv.Itoa(http.StatusLocked):                       "http.StatusLocked",
	strconv.Itoa(http.StatusFailedDependency):             "http.StatusFailedDependency",
	strconv.Itoa(http.StatusTooEarly):                     "http.StatusTooEarly",
	strconv.Itoa(http.StatusUpgradeRequired):              "http.StatusUpgradeRequired",
	strconv.Itoa(http.StatusPreconditionRequired):         "http.StatusPreconditionRequired",
	strconv.Itoa(http.StatusTooManyRequests):              "http.StatusTooManyRequests",
	strconv.Itoa(http.StatusRequestHeaderFieldsTooLarge):  "http.StatusRequestHeaderFieldsTooLarge",
	strconv.Itoa(http.StatusUnavailableForLegalReasons):   "http.StatusUnavailableForLegalReasons",

	strconv.Itoa(http.StatusInternalServerError):           "http.StatusInternalServerError",
	strconv.Itoa(http.StatusNotImplemented):                "http.StatusNotImplemented",
	strconv.Itoa(http.StatusBadGateway):                    "http.StatusBadGateway",
	strconv.Itoa(http.StatusServiceUnavailable):            "http.StatusServiceUnavailable",
	strconv.Itoa(http.StatusGatewayTimeout):                "http.StatusGatewayTimeout",
	strconv.Itoa(http.StatusHTTPVersionNotSupported):       "http.StatusHTTPVersionNotSupported",
	strconv.Itoa(http.StatusVariantAlsoNegotiates):         "http.StatusVariantAlsoNegotiates",
	strconv.Itoa(http.StatusInsufficientStorage):           "http.StatusInsufficientStorage",
	strconv.Itoa(http.StatusLoopDetected):                  "http.StatusLoopDetected",
	strconv.Itoa(http.StatusNotExtended):                   "http.StatusNotExtended",
	strconv.Itoa(http.StatusNetworkAuthenticationRequired): "http.StatusNetworkAuthenticationRequired",
}

var RPCDefaultPath = map[string]string{
	rpc.DefaultRPCPath:   "rpc.DefaultRPCPath",
	rpc.DefaultDebugPath: "rpc.DefaultDebugPath",
}

var TimeWeekday = map[string]string{
	time.Sunday.String():    "time.Sunday.String()",
	time.Monday.String():    "time.Monday.String()",
	time.Tuesday.String():   "time.Tuesday.String()",
	time.Wednesday.String(): "time.Wednesday.String()",
	time.Thursday.String():  "time.Thursday.String()",
	time.Friday.String():    "time.Friday.String()",
	time.Saturday.String():  "time.Saturday.String()",
}

var TimeMonth = map[string]string{
	time.January.String():   "time.January.String()",
	time.February.String():  "time.February.String()",
	time.March.String():     "time.March.String()",
	time.April.String():     "time.April.String()",
	time.May.String():       "time.May.String()",
	time.June.String():      "time.June.String()",
	time.July.String():      "time.July.String()",
	time.August.String():    "time.August.String()",
	time.September.String(): "time.September.String()",
	time.October.String():   "time.October.String()",
	time.November.String():  "time.November.String()",
	time.December.String():  "time.December.String()",
}

var TimeLayout = map[string]string{
	time.Layout:      "time.Layout",
	time.ANSIC:       "time.ANSIC",
	time.UnixDate:    "time.UnixDate",
	time.RubyDate:    "time.RubyDate",
	time.RFC822:      "time.RFC822",
	time.RFC822Z:     "time.RFC822Z",
	time.RFC850:      "time.RFC850",
	time.RFC1123:     "time.RFC1123",
	time.RFC1123Z:    "time.RFC1123Z",
	time.RFC3339:     "time.RFC3339",
	time.RFC3339Nano: "time.RFC3339Nano",
	time.Kitchen:     "time.Kitchen",
	time.Stamp:       "time.Stamp",
	time.StampMilli:  "time.StampMilli",
	time.StampMicro:  "time.StampMicro",
	time.StampNano:   "time.StampNano",
	time.DateTime:    "time.DateTime",
	time.DateOnly:    "time.DateOnly",
	time.TimeOnly:    "time.TimeOnly",
}

var SQLIsolationLevel = map[string]string{
	// sql.LevelDefault.String():         "sql.LevelDefault.String()",
	sql.LevelReadUncommitted.String(): "sql.LevelReadUncommitted.String()",
	sql.LevelReadCommitted.String():   "sql.LevelReadCommitted.String()",
	sql.LevelWriteCommitted.String():  "sql.LevelWriteCommitted.String()",
	sql.LevelRepeatableRead.String():  "sql.LevelRepeatableRead.String()",
	// sql.LevelSnapshot.String():        "sql.LevelSnapshot.String()",
	// sql.LevelSerializable.String():    "sql.LevelSerializable.String()",
	// sql.LevelLinearizable.String():    "sql.LevelLinearizable.String()",
}

var TLSSignatureScheme = map[string]string{
	tls.PSSWithSHA256.String():          "tls.PSSWithSHA256.String()",
	tls.ECDSAWithP256AndSHA256.String(): "tls.ECDSAWithP256AndSHA256.String()",
	tls.Ed25519.String():                "tls.Ed25519.String()",
	tls.PSSWithSHA384.String():          "tls.PSSWithSHA384.String()",
	tls.PSSWithSHA512.String():          "tls.PSSWithSHA512.String()",
	tls.PKCS1WithSHA256.String():        "tls.PKCS1WithSHA256.String()",
	tls.PKCS1WithSHA384.String():        "tls.PKCS1WithSHA384.String()",
	tls.PKCS1WithSHA512.String():        "tls.PKCS1WithSHA512.String()",
	tls.ECDSAWithP384AndSHA384.String(): "tls.ECDSAWithP384AndSHA384.String()",
	tls.ECDSAWithP521AndSHA512.String(): "tls.ECDSAWithP521AndSHA512.String()",
	tls.PKCS1WithSHA1.String():          "tls.PKCS1WithSHA1.String()",
	tls.ECDSAWithSHA1.String():          "tls.ECDSAWithSHA1.String()",
}

var ConstantKind = map[string]string{
	// constant.Unknown.String(): "constant.Unknown.String()",
	constant.Bool.String():    "constant.Bool.String()",
	constant.String.String():  "constant.String.String()",
	constant.Int.String():     "constant.Int.String()",
	constant.Float.String():   "constant.Float.String()",
	constant.Complex.String(): "constant.Complex.String()",
}

var TimeDateMonth = map[string]string{
	"1":  "time.January",
	"2":  "time.February",
	"3":  "time.March",
	"4":  "time.April",
	"5":  "time.May",
	"6":  "time.June",
	"7":  "time.July",
	"8":  "time.August",
	"9":  "time.September",
	"10": "time.October",
	"11": "time.November",
	"12": "time.December",
}
