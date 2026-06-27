package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/go-viper/mapstructure/v2"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	"github.com/golangci/golangci-lint/v2/pkg/exitcodes"
	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

type BaseConfig interface {
	IsInternalTest() bool
	SetConfigDir(dir string)
}

type BaseLoader struct {
	opts LoaderOptions

	viper *viper.Viper

	log logutils.Log

	cfg  BaseConfig
	args []string
}

func NewBaseLoader(log logutils.Log, v *viper.Viper, opts LoaderOptions, cfg BaseConfig, args []string) *BaseLoader {
	return &BaseLoader{
		opts:  opts,
		viper: v,
		log:   log,
		cfg:   cfg,
		args:  args,
	}
}

func (l *BaseLoader) Load() error {
	err := l.setConfigFile()
	if err != nil {
		return err
	}

	err = l.parseConfig()
	if err != nil {
		return err
	}

	return nil
}

func (l *BaseLoader) setConfigFile() error {
	configFile, err := l.evaluateOptions()
	if err != nil {
		if errors.Is(err, errConfigDisabled) {
			return nil
		}

		return fmt.Errorf("can't parse --config option: %w", err)
	}

	if configFile != "" {
		l.viper.SetConfigFile(configFile)

		// Assume YAML if the file has no extension.
		if filepath.Ext(configFile) == "" {
			l.viper.SetConfigType("yaml")
		}
	} else {
		l.setupConfigFileSearch()
	}

	return nil
}

func (l *BaseLoader) evaluateOptions() (string, error) {
	if l.opts.NoConfig && l.opts.Config != "" {
		return "", errors.New("can't combine option --config and --no-config")
	}

	if l.opts.NoConfig {
		return "", errConfigDisabled
	}

	configFile, err := homedir.Expand(l.opts.Config)
	if err != nil {
		return "", errors.New("failed to expand configuration path")
	}

	return configFile, nil
}

func (l *BaseLoader) setupConfigFileSearch() {
	l.viper.SetConfigName(".golangci")

	configSearchPaths := l.getConfigSearchPaths()

	l.log.Infof("Config search paths: %s", configSearchPaths)

	for _, p := range configSearchPaths {
		l.viper.AddConfigPath(p)
	}
}

func (l *BaseLoader) getConfigSearchPaths() []string {
	firstArg := "./..."
	if len(l.args) > 0 {
		firstArg = l.args[0]
	}

	absPath, err := filepath.Abs(firstArg)
	if err != nil {
		l.log.Warnf("Can't make abs path for %q: %s", firstArg, err)
		absPath = filepath.Clean(firstArg)
	}

	// start from it
	var currentDir string
	if fsutils.IsDir(absPath) {
		currentDir = absPath
	} else {
		currentDir = filepath.Dir(absPath)
	}

	// find all dirs from it up to the root
	searchPaths := []string{"./"}

	for {
		searchPaths = append(searchPaths, currentDir)

		parent := filepath.Dir(currentDir)
		if currentDir == parent || parent == "" {
			break
		}

		currentDir = parent
	}

	// find home directory for global config
	if home, err := homedir.Dir(); err != nil {
		l.log.Warnf("Can't get user's home directory: %v", err)
	} else if !slices.Contains(searchPaths, home) {
		searchPaths = append(searchPaths, home)
	}

	return searchPaths
}

func (l *BaseLoader) parseConfig() error {
	if err := l.viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// Load configuration from flags only.
			err = l.viper.Unmarshal(l.cfg, customDecoderHook())
			if err != nil {
				return fmt.Errorf("can't unmarshal config by viper (flags): %w", err)
			}

			return nil
		}

		return fmt.Errorf("can't read viper config: %w", err)
	}

	err := l.setConfigDir()
	if err != nil {
		return err
	}

	// Load configuration from all sources (flags, file).
	if err := l.viper.Unmarshal(l.cfg, customDecoderHook()); err != nil {
		return fmt.Errorf("can't unmarshal config by viper (flags, file): %w", err)
	}

	if l.cfg.IsInternalTest() { // just for testing purposes: to detect config file usage
		_, _ = fmt.Fprintln(logutils.StdOut, "test")
		os.Exit(exitcodes.Success)
	}

	return nil
}

func (l *BaseLoader) setConfigDir() error {
	usedConfigFile := l.viper.ConfigFileUsed()
	if usedConfigFile == "" {
		return nil
	}

	if usedConfigFile == os.Stdin.Name() {
		usedConfigFile = ""
		l.log.Infof("Reading config file stdin")
	} else {
		var err error
		usedConfigFile, err = fsutils.ShortestRelPath(usedConfigFile, "")
		if err != nil {
			l.log.Warnf("Can't pretty print config file path: %v", err)
		}

		l.log.Infof("Used config file %s", usedConfigFile)
	}

	usedConfigDir, err := filepath.Abs(filepath.Dir(usedConfigFile))
	if err != nil {
		return errors.New("can't get config directory")
	}

	l.cfg.SetConfigDir(usedConfigDir)

	return nil
}

func customDecoderHook() viper.DecoderConfigOption {
	return viper.DecodeHook(DecodeHookFunc())
}

func DecodeHookFunc() mapstructure.DecodeHookFunc {
	return mapstructure.ComposeDecodeHookFunc(
		// Default hooks (https://github.com/spf13/viper/blob/518241257478c557633ab36e474dfcaeb9a3c623/viper.go#L135-L138).
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),

		// Needed for forbidigo, and output.formats.
		mapstructure.TextUnmarshallerHookFunc(),
	)
}
