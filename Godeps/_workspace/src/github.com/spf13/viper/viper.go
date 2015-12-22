// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Viper is a application configuration system.
// It believes that applications can be configured a variety of ways
// via flags, ENVIRONMENT variables, configuration files retrieved
// from the file system, or a remote key/value store.

// Each item takes precedence over the item below it:

// overrides
// flag
// env
// config
// key/value store
// default

package viper

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/pflag"
	"github.com/kr/pretty"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cast"
	jww "github.com/spf13/jwalterweatherman"
	"gopkg.in/fsnotify.v1"
)

var v *Viper

func init() {
	v = New()
}

type remoteConfigFactory interface {
	Get(rp RemoteProvider) (io.Reader, error)
	Watch(rp RemoteProvider) (io.Reader, error)
}

// RemoteConfig is optional, see the remote package
var RemoteConfig remoteConfigFactory

// Denotes encountering an unsupported
// configuration filetype.
type UnsupportedConfigError string

// Returns the formatted configuration error.
func (str UnsupportedConfigError) Error() string {
	return fmt.Sprintf("Unsupported Config Type %q", string(str))
}

// Denotes encountering an unsupported remote
// provider. Currently only Etcd and Consul are
// supported.
type UnsupportedRemoteProviderError string

// Returns the formatted remote provider error.
func (str UnsupportedRemoteProviderError) Error() string {
	return fmt.Sprintf("Unsupported Remote Provider Type %q", string(str))
}

// Denotes encountering an error while trying to
// pull the configuration from the remote provider.
type RemoteConfigError string

// Returns the formatted remote provider error
func (rce RemoteConfigError) Error() string {
	return fmt.Sprintf("Remote Configurations Error: %s", string(rce))
}

// Denotes failing to find configuration file.
type ConfigFileNotFoundError struct {
	name, locations string
}

// Returns the formatted configuration error.
func (fnfe ConfigFileNotFoundError) Error() string {
	return fmt.Sprintf("Config File %q Not Found in %q", fnfe.name, fnfe.locations)
}

// Viper is a prioritized configuration registry. It
// maintains a set of configuration sources, fetches
// values to populate those, and provides them according
// to the source's priority.
// The priority of the sources is the following:
// 1. overrides
// 2. flags
// 3. env. variables
// 4. config file
// 5. key/value store
// 6. defaults
//
// For example, if values from the following sources were loaded:
//
//  Defaults : {
//  	"secret": "",
//  	"user": "default",
// 	    "endpoint": "https://localhost"
//  }
//  Config : {
//  	"user": "root"
//	    "secret": "defaultsecret"
//  }
//  Env : {
//  	"secret": "somesecretkey"
//  }
//
// The resulting config will have the following values:
//
//	{
//		"secret": "somesecretkey",
//		"user": "root",
//		"endpoint": "https://localhost"
//	}
type Viper struct {
	// Delimiter that separates a list of keys
	// used to access a nested value in one go
	keyDelim string

	// A set of paths to look for the config file in
	configPaths []string

	// A set of remote providers to search for the configuration
	remoteProviders []*defaultRemoteProvider

	// Name of file to look for inside the path
	configName string
	configFile string
	configType string
	envPrefix  string

	automaticEnvApplied bool
	envKeyReplacer      *strings.Replacer

	config         map[string]interface{}
	override       map[string]interface{}
	defaults       map[string]interface{}
	kvstore        map[string]interface{}
	pflags         map[string]*pflag.Flag
	env            map[string]string
	aliases        map[string]string
	typeByDefValue bool

	onConfigChange func(fsnotify.Event)
}

// Returns an initialized Viper instance.
func New() *Viper {
	v := new(Viper)
	v.keyDelim = "."
	v.configName = "config"
	v.config = make(map[string]interface{})
	v.override = make(map[string]interface{})
	v.defaults = make(map[string]interface{})
	v.kvstore = make(map[string]interface{})
	v.pflags = make(map[string]*pflag.Flag)
	v.env = make(map[string]string)
	v.aliases = make(map[string]string)
	v.typeByDefValue = false

	return v
}

// Intended for testing, will reset all to default settings.
// In the public interface for the viper package so applications
// can use it in their testing as well.
func Reset() {
	v = New()
	SupportedExts = []string{"json", "toml", "yaml", "yml"}
	SupportedRemoteProviders = []string{"etcd", "consul"}
}

type defaultRemoteProvider struct {
	provider      string
	endpoint      string
	path          string
	secretKeyring string
}

func (rp defaultRemoteProvider) Provider() string {
	return rp.provider
}

func (rp defaultRemoteProvider) Endpoint() string {
	return rp.endpoint
}

func (rp defaultRemoteProvider) Path() string {
	return rp.path
}

func (rp defaultRemoteProvider) SecretKeyring() string {
	return rp.secretKeyring
}

// RemoteProvider stores the configuration necessary
// to connect to a remote key/value store.
// Optional secretKeyring to unencrypt encrypted values
// can be provided.
type RemoteProvider interface {
	Provider() string
	Endpoint() string
	Path() string
	SecretKeyring() string
}

// Universally supported extensions.
var SupportedExts []string = []string{"json", "toml", "yaml", "yml", "properties", "props", "prop"}

// Universally supported remote providers.
var SupportedRemoteProviders []string = []string{"etcd", "consul"}

func OnConfigChange(run func(in fsnotify.Event)) { v.OnConfigChange(run) }
func (v *Viper) OnConfigChange(run func(in fsnotify.Event)) {
	v.onConfigChange = run
}

func WatchConfig() { v.WatchConfig() }
func (v *Viper) WatchConfig() {
	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}
		defer watcher.Close()

		done := make(chan bool)
		go func() {
			for {
				select {
				case event := <-watcher.Events:
					if event.Op&fsnotify.Write == fsnotify.Write {
						err := v.ReadInConfig()
						if err != nil {
							log.Println("error:", err)
						}
						v.onConfigChange(event)
					}
				case err := <-watcher.Errors:
					log.Println("error:", err)
				}
			}
		}()

		if v.configFile != "" {
			watcher.Add(v.configFile)
		} else {
			for _, x := range v.configPaths {
				err = watcher.Add(x)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
		<-done
	}()
}

// Explicitly define the path, name and extension of the config file
// Viper will use this and not check any of the config paths
func SetConfigFile(in string) { v.SetConfigFile(in) }
func (v *Viper) SetConfigFile(in string) {
	if in != "" {
		v.configFile = in
	}
}

// Define a prefix that ENVIRONMENT variables will use.
// E.g. if your prefix is "spf", the env registry
// will look for env. variables that start with "SPF_"
func SetEnvPrefix(in string) { v.SetEnvPrefix(in) }
func (v *Viper) SetEnvPrefix(in string) {
	if in != "" {
		v.envPrefix = in
	}
}

func (v *Viper) mergeWithEnvPrefix(in string) string {
	if v.envPrefix != "" {
		return strings.ToUpper(v.envPrefix + "_" + in)
	}

	return strings.ToUpper(in)
}

// TODO: should getEnv logic be moved into find(). Can generalize the use of
// rewriting keys many things, Ex: Get('someKey') -> some_key
// (cammel case to snake case for JSON keys perhaps)

// getEnv s a wrapper around os.Getenv which replaces characters in the original
// key. This allows env vars which have different keys then the config object
// keys
func (v *Viper) getEnv(key string) string {
	if v.envKeyReplacer != nil {
		key = v.envKeyReplacer.Replace(key)
	}
	return os.Getenv(key)
}

// Return the file used to populate the config registry
func ConfigFileUsed() string            { return v.ConfigFileUsed() }
func (v *Viper) ConfigFileUsed() string { return v.configFile }

// Add a path for Viper to search for the config file in.
// Can be called multiple times to define multiple search paths.
func AddConfigPath(in string) { v.AddConfigPath(in) }
func (v *Viper) AddConfigPath(in string) {
	if in != "" {
		absin := absPathify(in)
		jww.INFO.Println("adding", absin, "to paths to search")
		if !stringInSlice(absin, v.configPaths) {
			v.configPaths = append(v.configPaths, absin)
		}
	}
}

// AddRemoteProvider adds a remote configuration source.
// Remote Providers are searched in the order they are added.
// provider is a string value, "etcd" or "consul" are currently supported.
// endpoint is the url.  etcd requires http://ip:port  consul requires ip:port
// path is the path in the k/v store to retrieve configuration
// To retrieve a config file called myapp.json from /configs/myapp.json
// you should set path to /configs and set config name (SetConfigName()) to
// "myapp"
func AddRemoteProvider(provider, endpoint, path string) error {
	return v.AddRemoteProvider(provider, endpoint, path)
}
func (v *Viper) AddRemoteProvider(provider, endpoint, path string) error {
	if !stringInSlice(provider, SupportedRemoteProviders) {
		return UnsupportedRemoteProviderError(provider)
	}
	if provider != "" && endpoint != "" {
		jww.INFO.Printf("adding %s:%s to remote provider list", provider, endpoint)
		rp := &defaultRemoteProvider{
			endpoint: endpoint,
			provider: provider,
			path:     path,
		}
		if !v.providerPathExists(rp) {
			v.remoteProviders = append(v.remoteProviders, rp)
		}
	}
	return nil
}

// AddSecureRemoteProvider adds a remote configuration source.
// Secure Remote Providers are searched in the order they are added.
// provider is a string value, "etcd" or "consul" are currently supported.
// endpoint is the url.  etcd requires http://ip:port  consul requires ip:port
// secretkeyring is the filepath to your openpgp secret keyring.  e.g. /etc/secrets/myring.gpg
// path is the path in the k/v store to retrieve configuration
// To retrieve a config file called myapp.json from /configs/myapp.json
// you should set path to /configs and set config name (SetConfigName()) to
// "myapp"
// Secure Remote Providers are implemented with github.com/xordataexchange/crypt
func AddSecureRemoteProvider(provider, endpoint, path, secretkeyring string) error {
	return v.AddSecureRemoteProvider(provider, endpoint, path, secretkeyring)
}

func (v *Viper) AddSecureRemoteProvider(provider, endpoint, path, secretkeyring string) error {
	if !stringInSlice(provider, SupportedRemoteProviders) {
		return UnsupportedRemoteProviderError(provider)
	}
	if provider != "" && endpoint != "" {
		jww.INFO.Printf("adding %s:%s to remote provider list", provider, endpoint)
		rp := &defaultRemoteProvider{
			endpoint:      endpoint,
			provider:      provider,
			path:          path,
			secretKeyring: secretkeyring,
		}
		if !v.providerPathExists(rp) {
			v.remoteProviders = append(v.remoteProviders, rp)
		}
	}
	return nil
}

func (v *Viper) providerPathExists(p *defaultRemoteProvider) bool {
	for _, y := range v.remoteProviders {
		if reflect.DeepEqual(y, p) {
			return true
		}
	}
	return false
}

func (v *Viper) searchMap(source map[string]interface{}, path []string) interface{} {

	if len(path) == 0 {
		return source
	}

	if next, ok := source[path[0]]; ok {
		switch next.(type) {
		case map[interface{}]interface{}:
			return v.searchMap(cast.ToStringMap(next), path[1:])
		case map[string]interface{}:
			// Type assertion is safe here since it is only reached
			// if the type of `next` is the same as the type being asserted
			return v.searchMap(next.(map[string]interface{}), path[1:])
		default:
			return next
		}
	} else {
		return nil
	}
}

// SetTypeByDefaultValue enables or disables the inference of a key value's
// type when the Get function is used based upon a key's default value as
// opposed to the value returned based on the normal fetch logic.
//
// For example, if a key has a default value of []string{} and the same key
// is set via an environment variable to "a b c", a call to the Get function
// would return a string slice for the key if the key's type is inferred by
// the default value and the Get function would return:
//
//   []string {"a", "b", "c"}
//
// Otherwise the Get function would return:
//
//   "a b c"
func SetTypeByDefaultValue(enable bool) { v.SetTypeByDefaultValue(enable) }
func (v *Viper) SetTypeByDefaultValue(enable bool) {
	v.typeByDefValue = enable
}

// Viper is essentially repository for configurations
// Get can retrieve any value given the key to use
// Get has the behavior of returning the value associated with the first
// place from where it is set. Viper will check in the following order:
// override, flag, env, config file, key/value store, default
//
// Get returns an interface. For a specific value use one of the Get____ methods.
func Get(key string) interface{} { return v.Get(key) }
func (v *Viper) Get(key string) interface{} {
	path := strings.Split(key, v.keyDelim)

	lcaseKey := strings.ToLower(key)
	val := v.find(lcaseKey)

	if val == nil {
		source := v.find(path[0])
		if source != nil {
			if reflect.TypeOf(source).Kind() == reflect.Map {
				val = v.searchMap(cast.ToStringMap(source), path[1:])
			}
		}
	}

	// if no other value is returned and a flag does exist for the value,
	// get the flag's value even if the flag's value has not changed
	if val == nil {
		if flag, exists := v.pflags[lcaseKey]; exists {
			jww.TRACE.Println(key, "get pflag default", val)
			switch flag.Value.Type() {
			case "int", "int8", "int16", "int32", "int64":
				val = cast.ToInt(flag.Value.String())
			case "bool":
				val = cast.ToBool(flag.Value.String())
			default:
				val = flag.Value.String()
			}
		}
	}

	if val == nil {
		return nil
	}

	var valType interface{}
	if !v.typeByDefValue {
		valType = val
	} else {
		defVal, defExists := v.defaults[lcaseKey]
		if defExists {
			valType = defVal
		} else {
			valType = val
		}
	}

	switch valType.(type) {
	case bool:
		return cast.ToBool(val)
	case string:
		return cast.ToString(val)
	case int64, int32, int16, int8, int:
		return cast.ToInt(val)
	case float64, float32:
		return cast.ToFloat64(val)
	case time.Time:
		return cast.ToTime(val)
	case time.Duration:
		return cast.ToDuration(val)
	case []string:
		return cast.ToStringSlice(val)
	}
	return val
}

// Returns the value associated with the key as a string
func GetString(key string) string { return v.GetString(key) }
func (v *Viper) GetString(key string) string {
	return cast.ToString(v.Get(key))
}

// Returns the value associated with the key asa boolean
func GetBool(key string) bool { return v.GetBool(key) }
func (v *Viper) GetBool(key string) bool {
	return cast.ToBool(v.Get(key))
}

// Returns the value associated with the key as an integer
func GetInt(key string) int { return v.GetInt(key) }
func (v *Viper) GetInt(key string) int {
	return cast.ToInt(v.Get(key))
}

// Returns the value associated with the key as a float64
func GetFloat64(key string) float64 { return v.GetFloat64(key) }
func (v *Viper) GetFloat64(key string) float64 {
	return cast.ToFloat64(v.Get(key))
}

// Returns the value associated with the key as time
func GetTime(key string) time.Time { return v.GetTime(key) }
func (v *Viper) GetTime(key string) time.Time {
	return cast.ToTime(v.Get(key))
}

// Returns the value associated with the key as a duration
func GetDuration(key string) time.Duration { return v.GetDuration(key) }
func (v *Viper) GetDuration(key string) time.Duration {
	return cast.ToDuration(v.Get(key))
}

// Returns the value associated with the key as a slice of strings
func GetStringSlice(key string) []string { return v.GetStringSlice(key) }
func (v *Viper) GetStringSlice(key string) []string {
	return cast.ToStringSlice(v.Get(key))
}

// Returns the value associated with the key as a map of interfaces
func GetStringMap(key string) map[string]interface{} { return v.GetStringMap(key) }
func (v *Viper) GetStringMap(key string) map[string]interface{} {
	return cast.ToStringMap(v.Get(key))
}

// Returns the value associated with the key as a map of strings
func GetStringMapString(key string) map[string]string { return v.GetStringMapString(key) }
func (v *Viper) GetStringMapString(key string) map[string]string {
	return cast.ToStringMapString(v.Get(key))
}

// Returns the value associated with the key as a map to a slice of strings.
func GetStringMapStringSlice(key string) map[string][]string { return v.GetStringMapStringSlice(key) }
func (v *Viper) GetStringMapStringSlice(key string) map[string][]string {
	return cast.ToStringMapStringSlice(v.Get(key))
}

// Returns the size of the value associated with the given key
// in bytes.
func GetSizeInBytes(key string) uint { return v.GetSizeInBytes(key) }
func (v *Viper) GetSizeInBytes(key string) uint {
	sizeStr := cast.ToString(v.Get(key))
	return parseSizeInBytes(sizeStr)
}

// Takes a single key and unmarshals it into a Struct
func UnmarshalKey(key string, rawVal interface{}) error { return v.UnmarshalKey(key, rawVal) }
func (v *Viper) UnmarshalKey(key string, rawVal interface{}) error {
	return mapstructure.Decode(v.Get(key), rawVal)
}

// Unmarshals the config into a Struct. Make sure that the tags
// on the fields of the structure are properly set.
func Unmarshal(rawVal interface{}) error { return v.Unmarshal(rawVal) }
func (v *Viper) Unmarshal(rawVal interface{}) error {
	err := mapstructure.WeakDecode(v.AllSettings(), rawVal)

	if err != nil {
		return err
	}

	v.insensitiviseMaps()

	return nil
}

// Bind a full flag set to the configuration, using each flag's long
// name as the config key.
func BindPFlags(flags *pflag.FlagSet) (err error) { return v.BindPFlags(flags) }
func (v *Viper) BindPFlags(flags *pflag.FlagSet) (err error) {
	flags.VisitAll(func(flag *pflag.Flag) {
		if err = v.BindPFlag(flag.Name, flag); err != nil {
			return
		}
	})
	return nil
}

// Bind a specific key to a flag (as used by cobra)
// Example(where serverCmd is a Cobra instance):
//
//	 serverCmd.Flags().Int("port", 1138, "Port to run Application server on")
//	 Viper.BindPFlag("port", serverCmd.Flags().Lookup("port"))
//
func BindPFlag(key string, flag *pflag.Flag) (err error) { return v.BindPFlag(key, flag) }
func (v *Viper) BindPFlag(key string, flag *pflag.Flag) (err error) {
	if flag == nil {
		return fmt.Errorf("flag for %q is nil", key)
	}
	v.pflags[strings.ToLower(key)] = flag
	return nil
}

// Binds a Viper key to a ENV variable
// ENV variables are case sensitive
// If only a key is provided, it will use the env key matching the key, uppercased.
// EnvPrefix will be used when set when env name is not provided.
func BindEnv(input ...string) (err error) { return v.BindEnv(input...) }
func (v *Viper) BindEnv(input ...string) (err error) {
	var key, envkey string
	if len(input) == 0 {
		return fmt.Errorf("BindEnv missing key to bind to")
	}

	key = strings.ToLower(input[0])

	if len(input) == 1 {
		envkey = v.mergeWithEnvPrefix(key)
	} else {
		envkey = input[1]
	}

	v.env[key] = envkey

	return nil
}

// Given a key, find the value
// Viper will check in the following order:
// flag, env, config file, key/value store, default
// Viper will check to see if an alias exists first
func (v *Viper) find(key string) interface{} {
	var val interface{}
	var exists bool

	// if the requested key is an alias, then return the proper key
	key = v.realKey(key)

	// PFlag Override first
	flag, exists := v.pflags[key]
	if exists && flag.Changed {
		jww.TRACE.Println(key, "found in override (via pflag):", flag.Value)
		switch flag.Value.Type() {
		case "int", "int8", "int16", "int32", "int64":
			return cast.ToInt(flag.Value.String())
		case "bool":
			return cast.ToBool(flag.Value.String())
		default:
			return flag.Value.String()
		}
	}

	val, exists = v.override[key]
	if exists {
		jww.TRACE.Println(key, "found in override:", val)
		return val
	}

	if v.automaticEnvApplied {
		// even if it hasn't been registered, if automaticEnv is used,
		// check any Get request
		if val = v.getEnv(v.mergeWithEnvPrefix(key)); val != "" {
			jww.TRACE.Println(key, "found in environment with val:", val)
			return val
		}
	}

	envkey, exists := v.env[key]
	if exists {
		jww.TRACE.Println(key, "registered as env var", envkey)
		if val = v.getEnv(envkey); val != "" {
			jww.TRACE.Println(envkey, "found in environment with val:", val)
			return val
		} else {
			jww.TRACE.Println(envkey, "env value unset:")
		}
	}

	val, exists = v.config[key]
	if exists {
		jww.TRACE.Println(key, "found in config:", val)
		return val
	}

	// Test for nested config parameter
	if strings.Contains(key, v.keyDelim) {
		path := strings.Split(key, v.keyDelim)

		source := v.find(path[0])
		if source != nil {
			if reflect.TypeOf(source).Kind() == reflect.Map {
				val := v.searchMap(cast.ToStringMap(source), path[1:])
				jww.TRACE.Println(key, "found in nested config:", val)
				return val
			}
		}
	}

	val, exists = v.kvstore[key]
	if exists {
		jww.TRACE.Println(key, "found in key/value store:", val)
		return val
	}

	val, exists = v.defaults[key]
	if exists {
		jww.TRACE.Println(key, "found in defaults:", val)
		return val
	}

	return nil
}

// Check to see if the key has been set in any of the data locations
func IsSet(key string) bool { return v.IsSet(key) }
func (v *Viper) IsSet(key string) bool {
	t := v.Get(key)
	return t != nil
}

// Have Viper check ENV variables for all
// keys set in config, default & flags
func AutomaticEnv() { v.AutomaticEnv() }
func (v *Viper) AutomaticEnv() {
	v.automaticEnvApplied = true
}

// SetEnvKeyReplacer sets the strings.Replacer on the viper object
// Useful for mapping an environmental variable to a key that does
// not match it.
func SetEnvKeyReplacer(r *strings.Replacer) { v.SetEnvKeyReplacer(r) }
func (v *Viper) SetEnvKeyReplacer(r *strings.Replacer) {
	v.envKeyReplacer = r
}

// Aliases provide another accessor for the same key.
// This enables one to change a name without breaking the application
func RegisterAlias(alias string, key string) { v.RegisterAlias(alias, key) }
func (v *Viper) RegisterAlias(alias string, key string) {
	v.registerAlias(alias, strings.ToLower(key))
}

func (v *Viper) registerAlias(alias string, key string) {
	alias = strings.ToLower(alias)
	if alias != key && alias != v.realKey(key) {
		_, exists := v.aliases[alias]

		if !exists {
			// if we alias something that exists in one of the maps to another
			// name, we'll never be able to get that value using the original
			// name, so move the config value to the new realkey.
			if val, ok := v.config[alias]; ok {
				delete(v.config, alias)
				v.config[key] = val
			}
			if val, ok := v.kvstore[alias]; ok {
				delete(v.kvstore, alias)
				v.kvstore[key] = val
			}
			if val, ok := v.defaults[alias]; ok {
				delete(v.defaults, alias)
				v.defaults[key] = val
			}
			if val, ok := v.override[alias]; ok {
				delete(v.override, alias)
				v.override[key] = val
			}
			v.aliases[alias] = key
		}
	} else {
		jww.WARN.Println("Creating circular reference alias", alias, key, v.realKey(key))
	}
}

func (v *Viper) realKey(key string) string {
	newkey, exists := v.aliases[key]
	if exists {
		jww.DEBUG.Println("Alias", key, "to", newkey)
		return v.realKey(newkey)
	} else {
		return key
	}
}

// Check to see if the given key (or an alias) is in the config file
func InConfig(key string) bool { return v.InConfig(key) }
func (v *Viper) InConfig(key string) bool {
	// if the requested key is an alias, then return the proper key
	key = v.realKey(key)

	_, exists := v.config[key]
	return exists
}

// Set the default value for this key.
// Default only used when no value is provided by the user via flag, config or ENV.
func SetDefault(key string, value interface{}) { v.SetDefault(key, value) }
func (v *Viper) SetDefault(key string, value interface{}) {
	// If alias passed in, then set the proper default
	key = v.realKey(strings.ToLower(key))
	v.defaults[key] = value
}

// Sets the value for the key in the override regiser.
// Will be used instead of values obtained via
// flags, config file, ENV, default, or key/value store
func Set(key string, value interface{}) { v.Set(key, value) }
func (v *Viper) Set(key string, value interface{}) {
	// If alias passed in, then set the proper override
	key = v.realKey(strings.ToLower(key))
	v.override[key] = value
}

// Viper will discover and load the configuration file from disk
// and key/value stores, searching in one of the defined paths.
func ReadInConfig() error { return v.ReadInConfig() }
func (v *Viper) ReadInConfig() error {
	jww.INFO.Println("Attempting to read in config file")
	if !stringInSlice(v.getConfigType(), SupportedExts) {
		return UnsupportedConfigError(v.getConfigType())
	}

	file, err := ioutil.ReadFile(v.getConfigFile())
	if err != nil {
		return err
	}

	v.config = make(map[string]interface{})

	return v.unmarshalReader(bytes.NewReader(file), v.config)
}

func ReadConfig(in io.Reader) error { return v.ReadConfig(in) }
func (v *Viper) ReadConfig(in io.Reader) error {
	v.config = make(map[string]interface{})
	return v.unmarshalReader(in, v.config)
}

// func ReadBufConfig(buf *bytes.Buffer) error { return v.ReadBufConfig(buf) }
// func (v *Viper) ReadBufConfig(buf *bytes.Buffer) error {
// 	v.config = make(map[string]interface{})
// 	return v.unmarshalReader(buf, v.config)
// }

// Attempts to get configuration from a remote source
// and read it in the remote configuration registry.
func ReadRemoteConfig() error { return v.ReadRemoteConfig() }
func (v *Viper) ReadRemoteConfig() error {
	err := v.getKeyValueConfig()
	if err != nil {
		return err
	}
	return nil
}

func WatchRemoteConfig() error { return v.WatchRemoteConfig() }
func (v *Viper) WatchRemoteConfig() error {
	err := v.watchKeyValueConfig()
	if err != nil {
		return err
	}
	return nil
}

// Unmarshall a Reader into a map
// Should probably be an unexported function
func unmarshalReader(in io.Reader, c map[string]interface{}) error {
	return v.unmarshalReader(in, c)
}

func (v *Viper) unmarshalReader(in io.Reader, c map[string]interface{}) error {
	return unmarshallConfigReader(in, c, v.getConfigType())
}

func (v *Viper) insensitiviseMaps() {
	insensitiviseMap(v.config)
	insensitiviseMap(v.defaults)
	insensitiviseMap(v.override)
	insensitiviseMap(v.kvstore)
}

// retrieve the first found remote configuration
func (v *Viper) getKeyValueConfig() error {
	if RemoteConfig == nil {
		return RemoteConfigError("Enable the remote features by doing a blank import of the viper/remote package: '_ github.com/spf13/viper/remote'")
	}

	for _, rp := range v.remoteProviders {
		val, err := v.getRemoteConfig(rp)
		if err != nil {
			continue
		}
		v.kvstore = val
		return nil
	}
	return RemoteConfigError("No Files Found")
}

func (v *Viper) getRemoteConfig(provider *defaultRemoteProvider) (map[string]interface{}, error) {

	reader, err := RemoteConfig.Get(provider)
	if err != nil {
		return nil, err
	}
	err = v.unmarshalReader(reader, v.kvstore)
	return v.kvstore, err
}

// retrieve the first found remote configuration
func (v *Viper) watchKeyValueConfig() error {
	for _, rp := range v.remoteProviders {
		val, err := v.watchRemoteConfig(rp)
		if err != nil {
			continue
		}
		v.kvstore = val
		return nil
	}
	return RemoteConfigError("No Files Found")
}

func (v *Viper) watchRemoteConfig(provider *defaultRemoteProvider) (map[string]interface{}, error) {
	reader, err := RemoteConfig.Watch(provider)
	if err != nil {
		return nil, err
	}
	err = v.unmarshalReader(reader, v.kvstore)
	return v.kvstore, err
}

// Return all keys regardless where they are set
func AllKeys() []string { return v.AllKeys() }
func (v *Viper) AllKeys() []string {
	m := map[string]struct{}{}

	for key, _ := range v.defaults {
		m[key] = struct{}{}
	}

	for key, _ := range v.config {
		m[key] = struct{}{}
	}

	for key, _ := range v.kvstore {
		m[key] = struct{}{}
	}

	for key, _ := range v.override {
		m[key] = struct{}{}
	}

	a := []string{}
	for x, _ := range m {
		a = append(a, x)
	}

	return a
}

// Return all settings as a map[string]interface{}
func AllSettings() map[string]interface{} { return v.AllSettings() }
func (v *Viper) AllSettings() map[string]interface{} {
	m := map[string]interface{}{}
	for _, x := range v.AllKeys() {
		m[x] = v.Get(x)
	}

	return m
}

// Name for the config file.
// Does not include extension.
func SetConfigName(in string) { v.SetConfigName(in) }
func (v *Viper) SetConfigName(in string) {
	if in != "" {
		v.configName = in
	}
}

// Sets the type of the configuration returned by the
// remote source, e.g. "json".
func SetConfigType(in string) { v.SetConfigType(in) }
func (v *Viper) SetConfigType(in string) {
	if in != "" {
		v.configType = in
	}
}

func (v *Viper) getConfigType() string {
	if v.configType != "" {
		return v.configType
	}

	cf := v.getConfigFile()
	ext := filepath.Ext(cf)

	if len(ext) > 1 {
		return ext[1:]
	} else {
		return ""
	}
}

func (v *Viper) getConfigFile() string {
	// if explicitly set, then use it
	if v.configFile != "" {
		return v.configFile
	}

	cf, err := v.findConfigFile()
	if err != nil {
		return ""
	}

	v.configFile = cf
	return v.getConfigFile()
}

func (v *Viper) searchInPath(in string) (filename string) {
	jww.DEBUG.Println("Searching for config in ", in)
	for _, ext := range SupportedExts {
		jww.DEBUG.Println("Checking for", filepath.Join(in, v.configName+"."+ext))
		if b, _ := exists(filepath.Join(in, v.configName+"."+ext)); b {
			jww.DEBUG.Println("Found: ", filepath.Join(in, v.configName+"."+ext))
			return filepath.Join(in, v.configName+"."+ext)
		}
	}

	return ""
}

// search all configPaths for any config file.
// Returns the first path that exists (and is a config file)
func (v *Viper) findConfigFile() (string, error) {

	jww.INFO.Println("Searching for config in ", v.configPaths)

	for _, cp := range v.configPaths {
		file := v.searchInPath(cp)
		if file != "" {
			return file, nil
		}
	}
	return "", ConfigFileNotFoundError{v.configName, fmt.Sprintf("%s", v.configPaths)}
}

// Prints all configuration registries for debugging
// purposes.
func Debug() { v.Debug() }
func (v *Viper) Debug() {
	fmt.Println("Aliases:")
	pretty.Println(v.aliases)
	fmt.Println("Override:")
	pretty.Println(v.override)
	fmt.Println("PFlags")
	pretty.Println(v.pflags)
	fmt.Println("Env:")
	pretty.Println(v.env)
	fmt.Println("Key/Value Store:")
	pretty.Println(v.kvstore)
	fmt.Println("Config:")
	pretty.Println(v.config)
	fmt.Println("Defaults:")
	pretty.Println(v.defaults)
}
