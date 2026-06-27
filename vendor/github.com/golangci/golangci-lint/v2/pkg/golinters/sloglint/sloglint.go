package sloglint

import (
	"go-simpler.org/sloglint"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.SloglintSettings) *goanalysis.Linter {
	var opts *sloglint.Options

	if settings != nil {
		opts = &sloglint.Options{
			NoGlobalLogger:           settings.NoGlobal,
			ContextOnly:              settings.Context,
			StaticMessage:            settings.StaticMsg,
			MessageStyle:             settings.MsgStyle,
			NoMixedArguments:         settings.NoMixedArgs,
			KeyValuePairsOnly:        settings.KVOnly,
			AttributesOnly:           settings.AttrOnly,
			ArgumentsOnSeparateLines: settings.ArgsOnSepLines,
			ConstantKeys:             settings.NoRawKeys,
			AllowedKeys:              settings.AllowedKeys,
			ForbiddenKeys:            settings.ForbiddenKeys,
			KeyNamingCase:            settings.KeyNamingCase,
		}

		for _, fn := range settings.CustomFuncs {
			opts.CustomFuncs = append(opts.CustomFuncs, sloglint.Func{
				FullName:     fn.Name,
				MessagePos:   fn.MsgPos,
				ArgumentsPos: fn.ArgsPos,
			})
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(sloglint.New(opts)).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
