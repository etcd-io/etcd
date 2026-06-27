package gosec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const (
	// Globals are applicable to all rules and used for general
	// configuration settings for gosec.
	Globals = "global"
	// ExcludeRulesKey is the config key for path-based rule exclusions
	ExcludeRulesKey = "exclude-rules"
)

// GlobalOption defines the name of the global options
type GlobalOption string

const (
	// Nosec global option for #nosec directive
	Nosec GlobalOption = "nosec"
	// ShowIgnored defines whether nosec issues are counted as finding or not
	ShowIgnored GlobalOption = "show-ignored"
	// Audit global option which indicates that gosec runs in audit mode
	Audit GlobalOption = "audit"
	// NoSecAlternative global option alternative for #nosec directive
	NoSecAlternative GlobalOption = "#nosec"
	// ExcludeRules global option for some rules  should not be load
	ExcludeRules GlobalOption = "exclude"
	// IncludeRules global option for  should be load
	IncludeRules GlobalOption = "include"
	// SSA global option to enable go analysis framework with SSA support
	SSA GlobalOption = "ssa"
)

// NoSecTag returns the tag used to disable gosec for a line of code.
func NoSecTag(tag string) string {
	return fmt.Sprintf("%s%s", "#", tag)
}

// Config is used to provide configuration and customization to each of the rules.
type Config map[string]interface{}

// NewConfig initializes a new configuration instance. The configuration data then
// needs to be loaded via c.ReadFrom(strings.NewReader("config data"))
// or from a *os.File.
func NewConfig() Config {
	cfg := make(Config)
	cfg[Globals] = make(map[GlobalOption]string)
	return cfg
}

func (c Config) keyToGlobalOptions(key string) GlobalOption {
	return GlobalOption(key)
}

func (c Config) convertGlobals() {
	if globals, ok := c[Globals]; ok {
		if settings, ok := globals.(map[string]interface{}); ok {
			validGlobals := map[GlobalOption]string{}
			for k, v := range settings {
				validGlobals[c.keyToGlobalOptions(k)] = fmt.Sprintf("%v", v)
			}
			c[Globals] = validGlobals
		}
	}
}

// ReadFrom implements the io.ReaderFrom interface. This
// should be used with io.Reader to load configuration from
// file or from string etc.
func (c Config) ReadFrom(r io.Reader) (int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return int64(len(data)), err
	}
	if err = json.Unmarshal(data, &c); err != nil {
		return int64(len(data)), err
	}
	c.convertGlobals()
	return int64(len(data)), nil
}

// WriteTo implements the io.WriteTo interface. This should
// be used to save or print out the configuration information.
func (c Config) WriteTo(w io.Writer) (int64, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return int64(len(data)), err
	}
	return io.Copy(w, bytes.NewReader(data))
}

// Get returns the configuration section for the supplied key
func (c Config) Get(section string) (interface{}, error) {
	settings, found := c[section]
	if !found {
		return nil, fmt.Errorf("section %s not in configuration", section)
	}
	return settings, nil
}

// Set section in the configuration to specified value
func (c Config) Set(section string, value interface{}) {
	c[section] = value
}

// GetGlobal returns value associated with global configuration option
func (c Config) GetGlobal(option GlobalOption) (string, error) {
	if globals, ok := c[Globals]; ok {
		if settings, ok := globals.(map[GlobalOption]string); ok {
			if value, ok := settings[option]; ok {
				return value, nil
			}
			return "", fmt.Errorf("global setting for %s not found", option)
		}
	}
	return "", fmt.Errorf("no global config options found")
}

// SetGlobal associates a value with a global configuration option
func (c Config) SetGlobal(option GlobalOption, value string) {
	if globals, ok := c[Globals]; ok {
		if settings, ok := globals.(map[GlobalOption]string); ok {
			settings[option] = value
		}
	}
}

// IsGlobalEnabled checks if a global option is enabled
func (c Config) IsGlobalEnabled(option GlobalOption) (bool, error) {
	value, err := c.GetGlobal(option)
	if err != nil {
		return false, err
	}
	return (value == "true" || value == "enabled"), nil
}

// GetExcludeRules retrieves the path-based exclusion rules from the configuration.
// Returns nil if no exclusion rules are configured.
func (c Config) GetExcludeRules() ([]PathExcludeRule, error) {
	if c == nil {
		return nil, nil
	}

	rawRules, exists := c[ExcludeRulesKey]
	if !exists {
		return nil, nil
	}

	// The config is unmarshaled as map[string]interface{}, so we need to
	// re-marshal and unmarshal to get the proper typed struct
	rulesJSON, err := json.Marshal(rawRules)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal exclude-rules: %w", err)
	}

	var rules []PathExcludeRule
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse exclude-rules: %w", err)
	}

	return rules, nil
}

// SetExcludeRules sets the path-based exclusion rules in the configuration.
func (c Config) SetExcludeRules(rules []PathExcludeRule) {
	if c == nil {
		return
	}
	c[ExcludeRulesKey] = rules
}
