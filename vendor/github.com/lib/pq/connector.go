package pq

import (
	"context"
	"crypto/tls"
	"database/sql/driver"
	"fmt"
	"math/rand"
	"net"
	"net/netip"
	neturl "net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/lib/pq/internal/pgservice"
	"github.com/lib/pq/internal/pqutil"
	"github.com/lib/pq/internal/proto"
)

type (
	// SSLMode is a sslmode setting.
	SSLMode string

	// SSLNegotiation is a sslnegotiation setting.
	SSLNegotiation string

	// TargetSessionAttrs is a target_session_attrs setting.
	TargetSessionAttrs string

	// LoadBalanceHosts is a load_balance_hosts setting.
	LoadBalanceHosts string

	// ProtocolVersion is a min_protocol_version or max_protocol_version
	// setting.
	ProtocolVersion string

	// SSLProtocolVersion is a ssl_min_protocol_version or
	// ssl_max_protocol_version setting.
	SSLProtocolVersion string
)

// Values for [SSLMode] that pq supports.
const (
	// No SSL
	SSLModeDisable = SSLMode("disable")

	// First try a non-SSL connection and if that fails try an SSL connection.
	SSLModeAllow = SSLMode("allow")

	// First try an SSL connection and if that fails try a non-SSL connection.
	SSLModePrefer = SSLMode("prefer")

	// Require SSL, but skip verification. This is the default.
	SSLModeRequire = SSLMode("require")

	// Require SSL and verify that the certificate was signed by a trusted CA.
	SSLModeVerifyCA = SSLMode("verify-ca")

	// Require SSL and verify that the certificate was signed by a trusted CA
	// and the server host name matches the one in the certificate.
	SSLModeVerifyFull = SSLMode("verify-full")
)

var sslModes = []SSLMode{SSLModeDisable, SSLModeAllow, SSLModePrefer, SSLModeRequire,
	SSLModeVerifyFull, SSLModeVerifyCA}

func (s SSLMode) useSSL() bool {
	switch s {
	case SSLModePrefer, SSLModeRequire, SSLModeVerifyCA, SSLModeVerifyFull:
		return true
	}
	return false
}

// Values for [SSLNegotiation] that pq supports.
const (
	// Negotiate whether SSL should be used. This is the default.
	SSLNegotiationPostgres = SSLNegotiation("postgres")

	// Always use SSL, don't try to negotiate.
	SSLNegotiationDirect = SSLNegotiation("direct")
)

var sslNegotiations = []SSLNegotiation{SSLNegotiationPostgres, SSLNegotiationDirect}

// Values for [TargetSessionAttrs] that pq supports.
const (
	// Any successful connection is acceptable. This is the default.
	TargetSessionAttrsAny = TargetSessionAttrs("any")

	// Session must accept read-write transactions by default: the server must
	// not be in hot standby mode and default_transaction_read_only must be
	// off.
	TargetSessionAttrsReadWrite = TargetSessionAttrs("read-write")

	// Session must not accept read-write transactions by default.
	TargetSessionAttrsReadOnly = TargetSessionAttrs("read-only")

	// Server must not be in hot standby mode.
	TargetSessionAttrsPrimary = TargetSessionAttrs("primary")

	// Server must be in hot standby mode.
	TargetSessionAttrsStandby = TargetSessionAttrs("standby")

	// First try to find a standby server, but if none of the listed hosts is a
	// standby server, try again in any mode.
	TargetSessionAttrsPreferStandby = TargetSessionAttrs("prefer-standby")
)

var targetSessionAttrs = []TargetSessionAttrs{TargetSessionAttrsAny,
	TargetSessionAttrsReadWrite, TargetSessionAttrsReadOnly, TargetSessionAttrsPrimary,
	TargetSessionAttrsStandby, TargetSessionAttrsPreferStandby}

// Values for [LoadBalanceHosts] that pq supports.
const (
	// Don't load balance; try hosts in the order in which they're provided.
	// This is the default.
	LoadBalanceHostsDisable = LoadBalanceHosts("disable")

	// Hosts are tried in random order to balance connections across multiple
	// PostgreSQL servers.
	//
	// When using this value it's recommended to also configure a reasonable
	// value for connect_timeout. Because then, if one of the nodes that are
	// used for load balancing is not responding, a new node will be tried.
	LoadBalanceHostsRandom = LoadBalanceHosts("random")
)

var loadBalanceHosts = []LoadBalanceHosts{LoadBalanceHostsDisable, LoadBalanceHostsRandom}

// Values for [ProtocolVersion] that pq supports.
const (
	// ProtocolVersion30 is the default protocol version, supported in
	// PostgreSQL 3.0 and newer.
	ProtocolVersion30 = ProtocolVersion("3.0")

	// ProtocolVersion32 uses a longer secret key length for query cancellation,
	// supported in PostgreSQL 18 and newer.
	ProtocolVersion32 = ProtocolVersion("3.2")

	// ProtocolVersionLatest is the latest protocol version that pq supports
	// (which may not be supported by the server).
	ProtocolVersionLatest = ProtocolVersion("latest")
)

var protocolVersions = []ProtocolVersion{ProtocolVersion30, ProtocolVersion32, ProtocolVersionLatest}

// Values for [SSLProtocolVersion] that pq supports.
const (
	SSLProtocolVersionTLS10 = SSLProtocolVersion("TLSv1.0")
	SSLProtocolVersionTLS11 = SSLProtocolVersion("TLSv1.1")
	SSLProtocolVersionTLS12 = SSLProtocolVersion("TLSv1.2")
	SSLProtocolVersionTLS13 = SSLProtocolVersion("TLSv1.3")
)

var sslProtocolVersions = []SSLProtocolVersion{SSLProtocolVersionTLS10, SSLProtocolVersionTLS11,
	SSLProtocolVersionTLS12, SSLProtocolVersionTLS13}

func (s SSLProtocolVersion) tlsconf() uint16 {
	switch s {
	case SSLProtocolVersionTLS10:
		return tls.VersionTLS10
	case SSLProtocolVersionTLS11:
		return tls.VersionTLS11
	case SSLProtocolVersionTLS12:
		return tls.VersionTLS12
	case SSLProtocolVersionTLS13:
		return tls.VersionTLS13
	default:
		return 0
	}
}

// Connector represents a fixed configuration for the pq driver with a given
// dsn. Connector satisfies the [database/sql/driver.Connector] interface and
// can be used to create any number of DB Conn's via [sql.OpenDB].
type Connector struct {
	cfg    Config
	dialer Dialer
}

// NewConnector returns a connector for the pq driver in a fixed configuration
// with the given dsn. The returned connector can be used to create any number
// of equivalent Conn's. The returned connector is intended to be used with
// [sql.OpenDB].
func NewConnector(dsn string) (*Connector, error) {
	cfg, err := NewConfig(dsn)
	if err != nil {
		return nil, err
	}
	return NewConnectorConfig(cfg)
}

// NewConnectorConfig returns a connector for the pq driver in a fixed
// configuration with the given [Config]. The returned connector can be used to
// create any number of equivalent Conn's. The returned connector is intended to
// be used with [sql.OpenDB].
func NewConnectorConfig(cfg Config) (*Connector, error) {
	return &Connector{cfg: cfg, dialer: defaultDialer{}}, nil
}

// Connect returns a connection to the database using the fixed configuration of
// this Connector. Context is not used.
func (c *Connector) Connect(ctx context.Context) (driver.Conn, error) { return c.open(ctx) }

// Dialer allows change the dialer used to open connections.
func (c *Connector) Dialer(dialer Dialer) { c.dialer = dialer }

// Driver returns the underlying driver of this Connector.
func (c *Connector) Driver() driver.Driver { return &Driver{} }

func (p ProtocolVersion) proto() int {
	switch p {
	default:
		return proto.ProtocolVersion30
	case ProtocolVersion32, ProtocolVersionLatest:
		return proto.ProtocolVersion32
	}
}

// Config holds options pq supports when connecting to PostgreSQL.
//
// The postgres struct tag is used for the value from the DSN (e.g.
// "dbname=abc"), and the env struct tag is used for the environment variable
// (e.g. "PGDATABASE=abc")
type Config struct {
	// The host to connect to. Absolute paths and values that start with @ are
	// for unix domain sockets. Defaults to localhost.
	//
	// A comma-separated list of host names is also accepted, in which case each
	// host name in the list is tried in order or randomly if load_balance_hosts
	// is set. An empty item selects the default of localhost. The
	// target_session_attrs option controls properties the host must have to be
	// considered acceptable.
	Host string `postgres:"host" env:"PGHOST"`

	// IPv4 or IPv6 address to connect to. Using hostaddr allows the application
	// to avoid a host name lookup, which might be important in applications
	// with time constraints. A hostname is required for sslmode=verify-full and
	// the GSSAPI or SSPI authentication methods.
	//
	// The following rules are used:
	//
	// - If host is given without hostaddr, a host name lookup occurs.
	//
	// - If hostaddr is given without host, the value for hostaddr gives the
	//   server network address. The connection attempt will fail if the
	//   authentication method requires a host name.
	//
	// - If both host and hostaddr are given, the value for hostaddr gives the
	//   server network address. The value for host is ignored unless the
	//   authentication method requires it, in which case it will be used as the
	//   host name.
	//
	// A comma-separated list of hostaddr values is also accepted, in which case
	// each host in the list is tried in order or randonly if load_balance_hosts
	// is set. An empty item causes the corresponding host name to be used, or
	// the default host name if that is empty as well. The target_session_attrs
	// option controls properties the host must have to be considered
	// acceptable.
	Hostaddr netip.Addr `postgres:"hostaddr" env:"PGHOSTADDR"`

	// The port to connect to. Defaults to 5432.
	//
	// If multiple hosts were given in the host or hostaddr parameters, this
	// parameter may specify a comma-separated list of ports of the same length
	// as the host list, or it may specify a single port number to be used for
	// all hosts. An empty string, or an empty item in a comma-separated list,
	// specifies the default of 5432.
	Port uint16 `postgres:"port" env:"PGPORT"`

	// The name of the database to connect to.
	Database string `postgres:"dbname" env:"PGDATABASE"`

	// The user to sign in as. Defaults to the current user.
	User string `postgres:"user" env:"PGUSER"`

	// The user's password.
	Password string `postgres:"password" env:"PGPASSWORD"`

	// Path to [pgpass] file to store passwords; overrides Password.
	//
	// [pgpass]: http://www.postgresql.org/docs/current/static/libpq-pgpass.html
	Passfile string `postgres:"passfile" env:"PGPASSFILE"`

	// Commandline options to send to the server at connection start.
	Options string `postgres:"options" env:"PGOPTIONS"`

	// Application name, displayed in pg_stat_activity and log entries.
	ApplicationName string `postgres:"application_name" env:"PGAPPNAME"`

	// Used if application_name is not given. Specifying a fallback name is
	// useful in generic utility programs that wish to set a default application
	// name but allow it to be overridden by the user.
	FallbackApplicationName string `postgres:"fallback_application_name" env:"-"`

	// Whether to use SSL. Defaults to "require" (different from libpq's default
	// of "prefer").
	//
	// [RegisterTLSConfig] can be used to registers a custom [tls.Config], which
	// can be used by setting sslmode=pqgo-«key» in the connection string.
	SSLMode SSLMode `postgres:"sslmode" env:"PGSSLMODE"`

	// When set to "direct" it will use SSL without negotiation (PostgreSQL ≥17 only).
	SSLNegotiation SSLNegotiation `postgres:"sslnegotiation" env:"PGSSLNEGOTIATION"`

	// Path to client SSL certificate. The file must contain PEM encoded data.
	//
	// Defaults to ~/.postgresql/postgresql.crt
	SSLCert string `postgres:"sslcert" env:"PGSSLCERT"`

	// Path to secret key for sslcert. The file must contain PEM encoded data.
	//
	// Defaults to ~/.postgresql/postgresql.key
	SSLKey string `postgres:"sslkey" env:"PGSSLKEY"`

	// Path to root certificate. The file must contain PEM encoded data.
	//
	// The special value "system" can be used to load the system's root
	// certificates ([x509.SystemCertPool]). This will change the default
	// sslmode to verify-full and issue an error if a lower setting is used – as
	// anyone can register a valid certificate hostname verification becomes
	// essential.
	//
	// Defaults to ~/.postgresql/root.crt.
	SSLRootCert string `postgres:"sslrootcert" env:"PGSSLROOTCERT"`

	// By default SNI is on, any value which is not starting with "1" disables
	// SNI.
	SSLSNI bool `postgres:"sslsni" env:"PGSSLSNI"`

	// Minimum SSL/TLS protocol version to allow for the connection.
	//
	// The default is determined by [tls.Config.MinVersion], which is TLSv1.2 at
	// the time of writing.
	SSLMinProtocolVersion SSLProtocolVersion `postgres:"ssl_min_protocol_version" env:"SSLPGMINPROTOCOLVERSION"`

	// Maximum SSL/TLS protocol version to allow for the connection. If not set,
	// this parameter is ignored and the connection will use the maximum bound
	// defined by the backend, if set. Setting the maximum protocol version is
	// mainly useful for testing or if some component has issues working with a
	// newer protocol.
	SSLMaxProtocolVersion SSLProtocolVersion `postgres:"ssl_max_protocol_version" env:"SSLPGMAXPROTOCOLVERSION"`

	// Interpert sslcert and sslkey as PEM encoded data, rather than a path to a
	// PEM file. This is a pq extension, not supported in libpq.
	SSLInline bool `postgres:"sslinline" env:"-"`

	// GSS (Kerberos) service name when constructing the SPN (default is
	// postgres). This will be combined with the host to form the full SPN:
	// krbsrvname/host.
	KrbSrvname string `postgres:"krbsrvname" env:"PGKRBSRVNAME"`

	// GSS (Kerberos) SPN. This takes priority over krbsrvname if present. This
	// is a pq extension, not supported in libpq.
	KrbSpn string `postgres:"krbspn" env:"-"`

	// Maximum time to wait while connecting, in seconds. Zero, negative, or not
	// specified means wait indefinitely
	ConnectTimeout time.Duration `postgres:"connect_timeout" env:"PGCONNECT_TIMEOUT"`

	// Whether to always send []byte parameters over as binary. Enables single
	// round-trip mode for non-prepared Query calls. This is a pq extension, not
	// supported in libpq.
	BinaryParameters bool `postgres:"binary_parameters" env:"-"`

	// This connection should never use the binary format when receiving query
	// results from prepared statements. Only provided for debugging. This is a
	// pq extension, not supported in libpq.
	DisablePreparedBinaryResult bool `postgres:"disable_prepared_binary_result" env:"-"`

	// Client encoding; pq only supports UTF8 and this must be blank or "UTF8".
	ClientEncoding string `postgres:"client_encoding" env:"PGCLIENTENCODING"`

	// Date/time representation to use; pq only supports "ISO, MDY" and this
	// must be blank or "ISO, MDY".
	Datestyle string `postgres:"datestyle" env:"PGDATESTYLE"`

	// Default time zone.
	TZ string `postgres:"tz" env:"PGTZ"`

	// Default mode for the genetic query optimizer.
	Geqo string `postgres:"geqo" env:"PGGEQO"`

	// Determine whether the session must have certain properties to be
	// acceptable. It's typically used in combination with multiple host names
	// to select the first acceptable alternative among several hosts.
	TargetSessionAttrs TargetSessionAttrs `postgres:"target_session_attrs" env:"PGTARGETSESSIONATTRS"`

	// Controls the order in which the client tries to connect to the available
	// hosts. Once a connection attempt is successful no other hosts will be
	// tried. This parameter is typically used in combination with multiple host
	// names.
	//
	// This parameter can be used in combination with target_session_attrs to,
	// for example, load balance over standby servers only. Once successfully
	// connected, subsequent queries on the returned connection will all be sent
	// to the same server.
	LoadBalanceHosts LoadBalanceHosts `postgres:"load_balance_hosts" env:"PGLOADBALANCEHOSTS"`

	// Minimum acceptable PostgreSQL protocol version. If the server does not
	// support at least this version, the connection will fail. Defaults to
	// "3.0".
	MinProtocolVersion ProtocolVersion `postgres:"min_protocol_version" env:"PGMINPROTOCOLVERSION"`

	// Maximum PostgreSQL protocol version to request from the server. Defaults to "3.0".
	MaxProtocolVersion ProtocolVersion `postgres:"max_protocol_version" env:"PGMAXPROTOCOLVERSION"`

	// Load connection parameters from the service file at ~/.pg_service.conf
	// (which can be configured with PGSERVICEFILE).
	//
	// The service file is a INI-like file to configure connection parameters:
	//
	//   [servicename]
	//   # Comment
	//   dbname=foo
	//
	// Unlike libpq, this does not look at the system-wide service file, as the
	// location of this is a compile-time value that is not easy for pq to
	// retrieve.
	Service string `postgres:"service" env:"PGSERVICE"`

	// Path to connection service file. Defaults to ~/.pg_service.conf.
	ServiceFile string `postgres:"-" env:"PGSERVICEFILE"`

	// Runtime parameters: any unrecognized parameter in the DSN will be added
	// to this and sent to PostgreSQL during startup.
	Runtime map[string]string `postgres:"-" env:"-"`

	// Multi contains additional connection details. The first value is
	// available in [Config.Host], [Config.Hostaddr], and [Config.Port], and
	// additional ones (if any) are available here.
	Multi []ConfigMultihost

	// Record which parameters were given, so we can distinguish between an
	// empty string "not given at all".
	//
	// The alternative is to use pointers or sql.Null[..], but that's more
	// awkward to use.
	set []string `env:"set"`

	multiHost     []string
	multiHostaddr []netip.Addr
	multiPort     []uint16
}

// ConfigMultihost specifies an additional server to try to connect to.
type ConfigMultihost struct {
	Host     string
	Hostaddr netip.Addr
	Port     uint16
}

// NewConfig creates a new [Config] from the defaults, environment, service
// file, and DSN, in that order. That is: a service overrides any value from the
// environment, which in turn gets overridden by the same parameter in the
// connection string.
//
// Most connection parameters supported by PostgreSQL are supported; see the
// [Config] struct for supported parameters. pq also lets you specify any
// [run-time parameter] such as search_path or work_mem in the connection
// string. This is different from libpq, which uses the "options" parameter for
// this (which also works in pq).
//
// # key=value connection strings
//
// For key=value strings, use single quotes for values that contain whitespace
// or empty values. A backslash will escape the next character:
//
//	"user=pqgo password='with spaces'"
//	"user=''"
//	"user=space\ man password='it\'s valid'"
//
// # URL connection strings
//
// pq supports URL-style postgres:// or postgresql:// connection strings in the
// form:
//
//	postgres[ql]://[user[:pwd]@][net-location][:port][/dbname][?param1=value1&...]
//
// Go's [net/url.Parse] is more strict than PostgreSQL's URL parser and will
// (correctly) reject %2F in the host part. This means that unix-socket URLs:
//
//	postgres://[user[:pwd]@][unix-socket][:port[/dbname]][?param1=value1&...]
//	postgres://%2Ftmp%2Fpostgres/db
//
// will not work. You will need to use "host=/tmp/postgres dbname=db".
//
// Similarly, multiple ports also won't work, but ?port= will:
//
//	postgres://host1,host2:5432,6543/dbname         Doesn't work
//	postgres://host1,host2/dbname?port=5432,6543    Works
//
// # Environment
//
// Most [PostgreSQL environment variables] are supported by pq. Environment
// variables have a lower precedence than explicitly provided connection
// parameters. pq will return an error if environment variables it does not
// support are set. Environment variables have a lower precedence than
// explicitly provided connection parameters.
//
// [PostgreSQL environment variables]: http://www.postgresql.org/docs/current/static/libpq-envars.html
// [run-time parameter]: http://www.postgresql.org/docs/current/static/runtime-config.html
func NewConfig(dsn string) (Config, error) {
	return newConfig(dsn, os.Environ())
}

// Clone returns a copy of the [Config].
func (cfg Config) Clone() Config {
	rt := make(map[string]string)
	for k, v := range cfg.Runtime {
		rt[k] = v
	}
	c := cfg
	c.Runtime = rt
	c.set = append([]string{}, cfg.set...)
	return c
}

// hosts returns a slice of copies of this config, one for each host.
func (cfg Config) hosts() []Config {
	cfgs := make([]Config, 1, len(cfg.Multi)+1)
	cfgs[0] = cfg.Clone()
	for _, m := range cfg.Multi {
		c := cfg.Clone()
		c.Host, c.Hostaddr, c.Port = m.Host, m.Hostaddr, m.Port
		cfgs = append(cfgs, c)
	}

	if cfg.LoadBalanceHosts == LoadBalanceHostsRandom {
		rand.Shuffle(len(cfgs), func(i, j int) { cfgs[i], cfgs[j] = cfgs[j], cfgs[i] })
	}

	return cfgs
}

func newConfig(dsn string, env []string) (Config, error) {
	cfg := Config{
		Host:               "localhost",
		Port:               5432,
		SSLSNI:             true,
		MinProtocolVersion: "3.0",
		MaxProtocolVersion: "3.0",
	}
	if err := cfg.fromEnv(env); err != nil {
		return Config{}, err
	}
	if err := cfg.fromDSN(dsn); err != nil {
		return Config{}, err
	}
	if err := cfg.fromService(); err != nil {
		return Config{}, err
	}

	// Need to have exactly the same number of host and hostaddr, or only specify one.
	if cfg.isset("host") && cfg.Host != "" && cfg.Hostaddr != (netip.Addr{}) && len(cfg.multiHost) != len(cfg.multiHostaddr) {
		return Config{}, fmt.Errorf("pq: could not match %d host names to %d hostaddr values",
			len(cfg.multiHost)+1, len(cfg.multiHostaddr)+1)
	}
	// Need one port that applies to all or exactly the same number of ports as hosts.
	l, ll := max(len(cfg.multiHost), len(cfg.multiHostaddr)), len(cfg.multiPort)
	if l > 0 && ll > 0 && l != ll {
		return Config{}, fmt.Errorf("pq: could not match %d port numbers to %d hosts", ll+1, l+1)
	}

	// Populate Multi
	if len(cfg.multiHostaddr) > len(cfg.multiHost) {
		cfg.multiHost = make([]string, len(cfg.multiHostaddr))
	}
	for i, h := range cfg.multiHost {
		p := cfg.Port
		if len(cfg.multiPort) > 0 {
			p = cfg.multiPort[i]
		}
		var addr netip.Addr
		if len(cfg.multiHostaddr) > 0 {
			addr = cfg.multiHostaddr[i]
		}
		cfg.Multi = append(cfg.Multi, ConfigMultihost{
			Host:     h,
			Port:     p,
			Hostaddr: addr,
		})
	}

	// Use the "fallback" application name if necessary
	if cfg.isset("fallback_application_name") && !cfg.isset("application_name") {
		cfg.ApplicationName = cfg.FallbackApplicationName
	}

	// We can't work with any client_encoding other than UTF-8 currently.
	// However, we have historically allowed the user to set it to UTF-8
	// explicitly, and there's no reason to break such programs, so allow that.
	// Note that the "options" setting could also set client_encoding, but
	// parsing its value is not worth it.  Instead, we always explicitly send
	// client_encoding as a separate run-time parameter, which should override
	// anything set in options.
	if cfg.isset("client_encoding") && !isUTF8(cfg.ClientEncoding) {
		return Config{}, fmt.Errorf(`pq: unsupported client_encoding %q: must be absent or "UTF8"`, cfg.ClientEncoding)
	}
	// DateStyle needs a similar treatment.
	if cfg.isset("datestyle") && cfg.Datestyle != "ISO, MDY" {
		return Config{}, fmt.Errorf(`pq: unsupported datestyle %q: must be absent or "ISO, MDY"`, cfg.Datestyle)
	}
	cfg.ClientEncoding, cfg.Datestyle = "UTF8", "ISO, MDY"

	// Set default user if not explicitly provided.
	if !cfg.isset("user") {
		u, err := pqutil.User()
		if err != nil {
			return Config{}, err
		}
		cfg.User = u
	}

	// SSL is not necessary or supported over UNIX domain sockets.
	if nw, _ := cfg.network(); nw == "unix" {
		cfg.SSLMode = SSLModeDisable
	}

	if cfg.MinProtocolVersion > cfg.MaxProtocolVersion {
		return Config{}, fmt.Errorf("pq: min_protocol_version %q cannot be greater than max_protocol_version %q",
			cfg.MinProtocolVersion, cfg.MaxProtocolVersion)
	}
	if cfg.SSLNegotiation == SSLNegotiationDirect {
		switch cfg.SSLMode {
		case SSLModeDisable, SSLModeAllow, SSLModePrefer:
			return Config{}, fmt.Errorf(
				`pq: weak sslmode %q may not be used with sslnegotiation=direct (use "require", "verify-ca", or "verify-full")`,
				cfg.SSLMode)
		}
	}
	if cfg.SSLRootCert == "system" {
		if !cfg.isset("sslmode") {
			cfg.SSLMode = SSLModeVerifyFull
		}
		if cfg.SSLMode != SSLModeVerifyFull {
			return Config{}, fmt.Errorf(
				`pq: weak sslmode %q may not be used with sslrootcert=system (use "verify-full")`,
				cfg.SSLMode)
		}
	}

	return cfg, nil
}

func (cfg Config) network() (string, string) {
	if cfg.Hostaddr != (netip.Addr{}) {
		return "tcp", net.JoinHostPort(cfg.Hostaddr.String(), strconv.Itoa(int(cfg.Port)))
	}
	// UNIX domain sockets are either represented by an (absolute) file system
	// path or they live in the abstract name space (starting with an @).
	if filepath.IsAbs(cfg.Host) || strings.HasPrefix(cfg.Host, "@") {
		sockPath := filepath.Join(cfg.Host, ".s.PGSQL."+strconv.Itoa(int(cfg.Port)))
		return "unix", sockPath
	}
	return "tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port)))
}

func (cfg *Config) fromEnv(env []string) error {
	e := make(map[string]string)
	for _, v := range env {
		k, v, ok := strings.Cut(v, "=")
		if !ok {
			continue
		}
		switch k {
		case "PGREQUIRESSL", "PGSSLCOMPRESSION", // Deprecated.
			"PGREALM", "PGGSSENCMODE", "PGGSSDELEGATION", "PGGSSLIB", // krb stuff
			"PGREQUIREAUTH", "PGCHANNELBINDING",
			"PGSSLCERTMODE", "PGSSLCRL", "PGSSLCRLDIR", "PGREQUIREPEER":
			return fmt.Errorf("pq: environment variable $%s is not supported", k)
		case "PGKRBSRVNAME":
			if newGss == nil {
				return fmt.Errorf("pq: environment variable $%s is not supported as Kerberos is not enabled", k)
			}
		}
		e[k] = v
	}
	return cfg.setFromTag(e, "env", false)
}

// fromDSN parses the options from name and adds them to the values.
//
// The parsing code is based on conninfo_parse from libpq's fe-connect.c
func (cfg *Config) fromDSN(dsn string) error {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		var err error
		dsn, err = convertURL(dsn)
		if err != nil {
			return err
		}
	}

	var (
		opt  = make(map[string]string)
		s    = []rune(dsn)
		i    int
		next = func() (rune, bool) {
			if i >= len(s) {
				return 0, false
			}
			r := s[i]
			i++
			return r, true
		}
		skipSpaces = func() (rune, bool) {
			r, ok := next()
			for unicode.IsSpace(r) && ok {
				r, ok = next()
			}
			return r, ok
		}
	)

	for {
		var (
			keyRunes, valRunes []rune
			r                  rune
			ok                 bool
		)

		if r, ok = skipSpaces(); !ok {
			break
		}

		// Scan the key
		for !unicode.IsSpace(r) && r != '=' {
			keyRunes = append(keyRunes, r)
			if r, ok = next(); !ok {
				break
			}
		}

		// Skip any whitespace if we're not at the = yet
		if r != '=' {
			r, ok = skipSpaces()
		}

		// The current character should be =
		if r != '=' || !ok {
			return fmt.Errorf(`missing "=" after %q in connection info string`, string(keyRunes))
		}

		// Skip any whitespace after the =
		if r, ok = skipSpaces(); !ok {
			// If we reach the end here, the last value is just an empty string as per libpq.
			opt[string(keyRunes)] = ""
			break
		}

		if r != '\'' {
			for !unicode.IsSpace(r) {
				if r == '\\' {
					if r, ok = next(); !ok {
						return fmt.Errorf(`missing character after backslash`)
					}
				}
				valRunes = append(valRunes, r)

				if r, ok = next(); !ok {
					break
				}
			}
		} else {
		quote:
			for {
				if r, ok = next(); !ok {
					return fmt.Errorf(`unterminated quoted string literal in connection string`)
				}
				switch r {
				case '\'':
					break quote
				case '\\':
					r, _ = next()
					fallthrough
				default:
					valRunes = append(valRunes, r)
				}
			}
		}

		opt[string(keyRunes)] = string(valRunes)
	}

	return cfg.setFromTag(opt, "postgres", false)
}

func (cfg *Config) fromService() error {
	if cfg.Service == "" {
		return nil
	}

	if !cfg.isset("PGSERVICEFILE") {
		if home := pqutil.Home(false); home != "" {
			cfg.ServiceFile = filepath.Join(home, ".pg_service.conf")
		}
	}

	opts, err := pgservice.FindService(cfg.ServiceFile, cfg.Service)
	if err != nil {
		return fmt.Errorf("pq: %w", err)
	}
	return cfg.setFromTag(opts, "postgres", true)
}

func (cfg *Config) setFromTag(o map[string]string, tag string, service bool) error {
	f := "pq: wrong value for %q: "
	if tag == "env" {
		f = "pq: wrong value for $%s: "
	}
	var (
		types  = reflect.TypeOf(cfg).Elem()
		values = reflect.ValueOf(cfg).Elem()
	)
	for i := 0; i < types.NumField(); i++ {
		var (
			rt                    = types.Field(i)
			rv                    = values.Field(i)
			k                     = rt.Tag.Get(tag)
			connectTimeout        = (tag == "postgres" && k == "connect_timeout") || (tag == "env" && k == "PGCONNECT_TIMEOUT")
			host                  = (tag == "postgres" && k == "host") || (tag == "env" && k == "PGHOST")
			hostaddr              = (tag == "postgres" && k == "hostaddr") || (tag == "env" && k == "PGHOSTADDR")
			port                  = (tag == "postgres" && k == "port") || (tag == "env" && k == "PGPORT")
			sslmode               = (tag == "postgres" && k == "sslmode") || (tag == "env" && k == "PGSSLMODE")
			sslnegotiation        = (tag == "postgres" && k == "sslnegotiation") || (tag == "env" && k == "PGSSLNEGOTIATION")
			targetsessionattrs    = (tag == "postgres" && k == "target_session_attrs") || (tag == "env" && k == "PGTARGETSESSIONATTRS")
			loadbalancehosts      = (tag == "postgres" && k == "load_balance_hosts") || (tag == "env" && k == "PGLOADBALANCEHOSTS")
			minprotocolversion    = (tag == "postgres" && k == "min_protocol_version") || (tag == "env" && k == "PGMINPROTOCOLVERSION")
			maxprotocolversion    = (tag == "postgres" && k == "max_protocol_version") || (tag == "env" && k == "PGMAXPROTOCOLVERSION")
			sslminprotocolversion = (tag == "postgres" && k == "ssl_min_protocol_version") || (tag == "env" && k == "SSLPGMINPROTOCOLVERSION")
			sslmaxprotocolversion = (tag == "postgres" && k == "ssl_max_protocol_version") || (tag == "env" && k == "SSLPGMAXPROTOCOLVERSION")
		)
		if k == "" || k == "-" {
			continue
		}

		v, ok := o[k]
		delete(o, k)
		if ok {
			t, ok := rt.Tag.Lookup("postgres")
			if !ok || t == "" || t == "-" { // For PGSERVICEFILE, which can only be from env
				t, ok = rt.Tag.Lookup("env")
			}
			if ok && t != "" && t != "-" {
				cfg.set = append(cfg.set, t)
			}
			switch rt.Type.Kind() {
			default:
				return fmt.Errorf("don't know how to set %s: unknown type %s", rt.Name, rt.Type.Kind())
			case reflect.Struct:
				if rt.Type == reflect.TypeOf(netip.Addr{}) {
					if hostaddr {
						vv := strings.Split(v, ",")
						v = vv[0]
						for _, vvv := range vv[1:] {
							if vvv == "" {
								cfg.multiHostaddr = append(cfg.multiHostaddr, netip.Addr{})
							} else {
								ip, err := netip.ParseAddr(vvv)
								if err != nil {
									return fmt.Errorf(f+"%w", k, err)
								}
								cfg.multiHostaddr = append(cfg.multiHostaddr, ip)
							}
						}
					}
					ip, err := netip.ParseAddr(v)
					if err != nil {
						return fmt.Errorf(f+"%w", k, err)
					}
					rv.Set(reflect.ValueOf(ip))
				} else {
					return fmt.Errorf("don't know how to set %s: unknown type %s", rt.Name, rt.Type)
				}
			case reflect.String:
				if sslmode && !slices.Contains(sslModes, SSLMode(v)) && !(strings.HasPrefix(v, "pqgo-") && hasTLSConfig(v[5:])) {
					return fmt.Errorf(f+`%q is not supported; supported values are %s`, k, v, pqutil.Join(sslModes))
				}
				if sslnegotiation && !slices.Contains(sslNegotiations, SSLNegotiation(v)) {
					return fmt.Errorf(f+`%q is not supported; supported values are %s`, k, v, pqutil.Join(sslNegotiations))
				}
				if targetsessionattrs && !slices.Contains(targetSessionAttrs, TargetSessionAttrs(v)) {
					return fmt.Errorf(f+`%q is not supported; supported values are %s`, k, v, pqutil.Join(targetSessionAttrs))
				}
				if loadbalancehosts && !slices.Contains(loadBalanceHosts, LoadBalanceHosts(v)) {
					return fmt.Errorf(f+`%q is not supported; supported values are %s`, k, v, pqutil.Join(loadBalanceHosts))
				}
				if (minprotocolversion || maxprotocolversion) && !slices.Contains(protocolVersions, ProtocolVersion(v)) {
					return fmt.Errorf(f+`%q is not supported; supported values are %s`, k, v, pqutil.Join(protocolVersions))
				}
				if (sslminprotocolversion || sslmaxprotocolversion) && !slices.Contains(sslProtocolVersions, SSLProtocolVersion(v)) {
					return fmt.Errorf(f+`%q is not supported; supported values are %s`, k, v, pqutil.Join(sslProtocolVersions))
				}
				if host {
					vv := strings.Split(v, ",")
					v = vv[0]
					for i, vvv := range vv[1:] {
						if vvv == "" {
							vv[i+1] = "localhost"
						}
					}
					cfg.multiHost = append(cfg.multiHost, vv[1:]...)
				}
				rv.SetString(v)
			case reflect.Int64:
				n, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return fmt.Errorf(f+"%w", k, err)
				}
				if connectTimeout {
					n = int64(time.Duration(n) * time.Second)
				}
				rv.SetInt(n)
			case reflect.Uint16:
				if port {
					vv := strings.Split(v, ",")
					v = vv[0]
					for _, vvv := range vv[1:] {
						if vvv == "" {
							vvv = "5432"
						}
						n, err := strconv.ParseUint(vvv, 10, 16)
						if err != nil {
							return fmt.Errorf(f+"%w", k, err)
						}
						cfg.multiPort = append(cfg.multiPort, uint16(n))
					}
				}
				n, err := strconv.ParseUint(v, 10, 16)
				if err != nil {
					return fmt.Errorf(f+"%w", k, err)
				}
				rv.SetUint(n)
			case reflect.Bool:
				b, err := pqutil.ParseBool(v)
				if err != nil {
					return fmt.Errorf(f+"%w", k, err)
				}
				rv.SetBool(b)
			}
		}
	}

	if service && len(o) > 0 {
		// TODO(go1.23): use maps.Keys once we require Go 1.23.
		var key string
		for k := range o {
			key = k
			break
		}
		return fmt.Errorf("pq: unknown setting %q in service file for service %q", key, cfg.Service)
	}

	// Set run-time; we delete map keys as they're set in the struct.
	if !service && tag == "postgres" {
		// Make sure database= sets dbname=, as that previously worked (kind of
		// by accident).
		// TODO(v2): remove
		if d, ok := o["database"]; ok {
			cfg.Database = d
			delete(o, "database")
		}
		cfg.Runtime = o
	}

	return nil
}

// Should generally only be used from newConfig(), as it will never be set if
// people go outside that.
func (cfg Config) isset(name string) bool {
	return slices.Contains(cfg.set, name)
}

// Convert to a map; used only in tests.
func (cfg Config) tomap() map[string]string {
	var (
		o      = make(map[string]string)
		values = reflect.ValueOf(cfg)
		types  = reflect.TypeOf(cfg)
	)
	for i := 0; i < types.NumField(); i++ {
		var (
			rt = types.Field(i)
			rv = values.Field(i)
			k  = rt.Tag.Get("postgres")
		)
		if k == "" || k == "-" {
			continue
		}
		if !rv.IsZero() || slices.Contains(cfg.set, k) {
			switch rt.Type.Kind() {
			default:
				if s, ok := rv.Interface().(fmt.Stringer); ok {
					o[k] = s.String()
				} else {
					o[k] = rv.String()
				}
			case reflect.Uint16:
				n := rv.Uint()
				o[k] = strconv.FormatUint(n, 10)
			case reflect.Int64:
				n := rv.Int()
				if k == "connect_timeout" {
					n = int64(time.Duration(n) / time.Second)
				}
				o[k] = strconv.FormatInt(n, 10)
			case reflect.Bool:
				if rv.Bool() {
					o[k] = "yes"
				} else {
					o[k] = "no"
				}
			}
		}
	}
	for k, v := range cfg.Runtime {
		o[k] = v
	}
	return o
}

// Create DSN for this config; used only in tests.
func (cfg Config) string() string {
	var (
		m    = cfg.tomap()
		keys = make([]string, 0, len(m))
	)
	for k := range m {
		switch k {
		case "datestyle", "client_encoding":
			continue
		case "host", "port", "user", "sslsni", "min_protocol_version", "max_protocol_version":
			if !cfg.isset(k) {
				continue
			}
		}
		if k == "application_name" && m[k] == "pqgo" {
			continue
		}
		if k == "host" && len(cfg.multiHost) > 0 {
			m[k] += "," + strings.Join(cfg.multiHost, ",")
		}
		if k == "hostaddr" && len(cfg.multiHostaddr) > 0 {
			for _, ha := range cfg.multiHostaddr {
				m[k] += ","
				if ha != (netip.Addr{}) {
					m[k] += ha.String()
				}
			}
		}
		if k == "port" && len(cfg.multiPort) > 0 {
			for _, p := range cfg.multiPort {
				m[k] += "," + strconv.Itoa(int(p))
			}
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteByte('=')
		var (
			v     = m[k]
			nv    = make([]rune, 0, len(v)+2)
			quote = v == ""
		)
		for _, c := range v {
			if c == ' ' {
				quote = true
			}
			if c == '\'' {
				nv = append(nv, '\\')
			}
			nv = append(nv, c)
		}
		if quote {
			b.WriteByte('\'')
		}
		b.WriteString(string(nv))
		if quote {
			b.WriteByte('\'')
		}
	}
	return b.String()
}

// Recognize all sorts of silly things as "UTF-8", like Postgres does
func isUTF8(name string) bool {
	s := strings.Map(func(c rune) rune {
		if 'A' <= c && c <= 'Z' {
			return c + ('a' - 'A')
		}
		if 'a' <= c && c <= 'z' || '0' <= c && c <= '9' {
			return c
		}
		return -1 // discard
	}, name)
	return s == "utf8" || s == "unicode"
}

func convertURL(url string) (string, error) {
	u, err := neturl.Parse(url)
	if err != nil {
		return "", err
	}

	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", fmt.Errorf("invalid connection protocol: %s", u.Scheme)
	}

	var kvs []string
	escaper := strings.NewReplacer(`'`, `\'`, `\`, `\\`)
	accrue := func(k, v string) {
		if v != "" {
			kvs = append(kvs, k+"='"+escaper.Replace(v)+"'")
		}
	}

	if u.User != nil {
		pw, _ := u.User.Password()
		accrue("user", u.User.Username())
		accrue("password", pw)
	}

	if host, port, err := net.SplitHostPort(u.Host); err != nil {
		accrue("host", u.Host)
	} else {
		accrue("host", host)
		accrue("port", port)
	}

	if u.Path != "" {
		accrue("dbname", u.Path[1:])
	}

	q := u.Query()
	for k := range q {
		accrue(k, q.Get(k))
	}

	sort.Strings(kvs) // Makes testing easier (not a performance concern)
	return strings.Join(kvs, " "), nil
}
