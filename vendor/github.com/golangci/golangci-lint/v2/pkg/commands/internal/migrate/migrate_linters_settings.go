package migrate

import (
	"slices"
	"strings"

	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/ptr"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/versionone"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/versiontwo"
)

func toLinterSettings(old versionone.LintersSettings) versiontwo.LintersSettings {
	return versiontwo.LintersSettings{
		Asasalint:       toAsasalintSettings(old.Asasalint),
		BiDiChk:         toBiDiChkSettings(old.BiDiChk),
		CopyLoopVar:     toCopyLoopVarSettings(old.CopyLoopVar),
		Cyclop:          toCyclopSettings(old.Cyclop),
		Decorder:        toDecorderSettings(old.Decorder),
		Depguard:        toDepGuardSettings(old.Depguard),
		Dogsled:         toDogsledSettings(old.Dogsled),
		Dupl:            toDuplSettings(old.Dupl),
		DupWord:         toDupWordSettings(old.DupWord),
		Errcheck:        toErrcheckSettings(old.Errcheck),
		ErrChkJSON:      toErrChkJSONSettings(old.ErrChkJSON),
		ErrorLint:       toErrorLintSettings(old.ErrorLint),
		Exhaustive:      toExhaustiveSettings(old.Exhaustive),
		Exhaustruct:     toExhaustructSettings(old.Exhaustruct),
		Fatcontext:      toFatcontextSettings(old.Fatcontext),
		Forbidigo:       toForbidigoSettings(old.Forbidigo),
		Funlen:          toFunlenSettings(old.Funlen),
		GinkgoLinter:    toGinkgoLinterSettings(old.GinkgoLinter),
		Gocognit:        toGocognitSettings(old.Gocognit),
		GoChecksumType:  toGoChecksumTypeSettings(old.GoChecksumType),
		Goconst:         toGoConstSettings(old.Goconst),
		Gocritic:        toGoCriticSettings(old.Gocritic),
		Gocyclo:         toGoCycloSettings(old.Gocyclo),
		Godot:           toGodotSettings(old.Godot),
		Godox:           toGodoxSettings(old.Godox),
		Goheader:        toGoHeaderSettings(old.Goheader),
		GoModDirectives: toGoModDirectivesSettings(old.GoModDirectives),
		Gomodguard:      toGoModGuardSettings(old.Gomodguard),
		Gosec:           toGoSecSettings(old.Gosec),
		Gosmopolitan:    toGosmopolitanSettings(old.Gosmopolitan),
		Govet:           toGovetSettings(old.Govet),
		Grouper:         toGrouperSettings(old.Grouper),
		Iface:           toIfaceSettings(old.Iface),
		ImportAs:        toImportAsSettings(old.ImportAs),
		Inamedparam:     toINamedParamSettings(old.Inamedparam),
		InterfaceBloat:  toInterfaceBloatSettings(old.InterfaceBloat),
		Ireturn:         toIreturnSettings(old.Ireturn),
		Lll:             toLllSettings(old.Lll),
		LoggerCheck:     toLoggerCheckSettings(old.LoggerCheck),
		MaintIdx:        toMaintIdxSettings(old.MaintIdx),
		Makezero:        toMakezeroSettings(old.Makezero),
		Misspell:        toMisspellSettings(old.Misspell),
		Mnd:             toMndSettings(old.Mnd),
		MustTag:         toMustTagSettings(old.MustTag),
		Nakedret:        toNakedretSettings(old.Nakedret),
		Nestif:          toNestifSettings(old.Nestif),
		NilNil:          toNilNilSettings(old.NilNil),
		Nlreturn:        toNlreturnSettings(old.Nlreturn),
		NoLintLint:      toNoLintLintSettings(old.NoLintLint),
		NoNamedReturns:  toNoNamedReturnsSettings(old.NoNamedReturns),
		ParallelTest:    toParallelTestSettings(old.ParallelTest),
		PerfSprint:      toPerfSprintSettings(old.PerfSprint),
		Prealloc:        toPreallocSettings(old.Prealloc),
		Predeclared:     toPredeclaredSettings(old.Predeclared),
		Promlinter:      toPromlinterSettings(old.Promlinter),
		ProtoGetter:     toProtoGetterSettings(old.ProtoGetter),
		Reassign:        toReassignSettings(old.Reassign),
		Recvcheck:       toRecvcheckSettings(old.Recvcheck),
		Revive:          toReviveSettings(old.Revive),
		RowsErrCheck:    toRowsErrCheckSettings(old.RowsErrCheck),
		SlogLint:        toSlogLintSettings(old.SlogLint),
		Spancheck:       toSpancheckSettings(old.Spancheck),
		Staticcheck:     toStaticCheckSettings(old),
		TagAlign:        toTagAlignSettings(old.TagAlign),
		Tagliatelle:     toTagliatelleSettings(old.Tagliatelle),
		Testifylint:     toTestifylintSettings(old.Testifylint),
		Testpackage:     toTestpackageSettings(old.Testpackage),
		Thelper:         toThelperSettings(old.Thelper),
		Unconvert:       toUnconvertSettings(old.Unconvert),
		Unparam:         toUnparamSettings(old.Unparam),
		Unused:          toUnusedSettings(old.Unused),
		UseStdlibVars:   toUseStdlibVarsSettings(old.UseStdlibVars),
		UseTesting:      toUseTestingSettings(old.UseTesting),
		Varnamelen:      toVarnamelenSettings(old.Varnamelen),
		Whitespace:      toWhitespaceSettings(old.Whitespace),
		Wrapcheck:       toWrapcheckSettings(old.Wrapcheck),
		WSL:             toWSLSettings(old.WSL),
		Custom:          toCustom(old.Custom),
	}
}

func toAsasalintSettings(old versionone.AsasalintSettings) versiontwo.AsasalintSettings {
	return versiontwo.AsasalintSettings{
		Exclude:              old.Exclude,
		UseBuiltinExclusions: old.UseBuiltinExclusions,
	}
}

func toBiDiChkSettings(old versionone.BiDiChkSettings) versiontwo.BiDiChkSettings {
	// The values are true be default, but the default are defined after the configuration loading.
	// So the serialization doesn't have good results, but it's complex to do better.
	return versiontwo.BiDiChkSettings{
		LeftToRightEmbedding:     old.LeftToRightEmbedding,
		RightToLeftEmbedding:     old.RightToLeftEmbedding,
		PopDirectionalFormatting: old.PopDirectionalFormatting,
		LeftToRightOverride:      old.LeftToRightOverride,
		RightToLeftOverride:      old.RightToLeftOverride,
		LeftToRightIsolate:       old.LeftToRightIsolate,
		RightToLeftIsolate:       old.RightToLeftIsolate,
		FirstStrongIsolate:       old.FirstStrongIsolate,
		PopDirectionalIsolate:    old.PopDirectionalIsolate,
	}
}

func toCopyLoopVarSettings(old versionone.CopyLoopVarSettings) versiontwo.CopyLoopVarSettings {
	return versiontwo.CopyLoopVarSettings{
		CheckAlias: old.CheckAlias,
	}
}

func toCyclopSettings(old versionone.Cyclop) versiontwo.CyclopSettings {
	return versiontwo.CyclopSettings{
		MaxComplexity:  old.MaxComplexity,
		PackageAverage: old.PackageAverage,
	}
}

func toDecorderSettings(old versionone.DecorderSettings) versiontwo.DecorderSettings {
	return versiontwo.DecorderSettings{
		DecOrder:                  old.DecOrder,
		IgnoreUnderscoreVars:      old.IgnoreUnderscoreVars,
		DisableDecNumCheck:        old.DisableDecNumCheck,
		DisableTypeDecNumCheck:    old.DisableTypeDecNumCheck,
		DisableConstDecNumCheck:   old.DisableConstDecNumCheck,
		DisableVarDecNumCheck:     old.DisableVarDecNumCheck,
		DisableDecOrderCheck:      old.DisableDecOrderCheck,
		DisableInitFuncFirstCheck: old.DisableInitFuncFirstCheck,
	}
}

func toDepGuardSettings(old versionone.DepGuardSettings) versiontwo.DepGuardSettings {
	settings := versiontwo.DepGuardSettings{}

	for k, r := range old.Rules {
		if settings.Rules == nil {
			settings.Rules = make(map[string]*versiontwo.DepGuardList)
		}

		list := &versiontwo.DepGuardList{
			ListMode: r.ListMode,
			Files:    r.Files,
			Allow:    r.Allow,
		}

		for _, deny := range r.Deny {
			list.Deny = append(list.Deny, versiontwo.DepGuardDeny{
				Pkg:  deny.Pkg,
				Desc: deny.Desc,
			})
		}

		settings.Rules[k] = list
	}

	return settings
}

func toDogsledSettings(old versionone.DogsledSettings) versiontwo.DogsledSettings {
	return versiontwo.DogsledSettings{
		MaxBlankIdentifiers: old.MaxBlankIdentifiers,
	}
}

func toDuplSettings(old versionone.DuplSettings) versiontwo.DuplSettings {
	return versiontwo.DuplSettings{
		Threshold: old.Threshold,
	}
}

func toDupWordSettings(old versionone.DupWordSettings) versiontwo.DupWordSettings {
	return versiontwo.DupWordSettings{
		Keywords: old.Keywords,
		Ignore:   old.Ignore,
	}
}

func toErrcheckSettings(old versionone.ErrcheckSettings) versiontwo.ErrcheckSettings {
	return versiontwo.ErrcheckSettings{
		DisableDefaultExclusions: old.DisableDefaultExclusions,
		CheckTypeAssertions:      old.CheckTypeAssertions,
		CheckAssignToBlank:       old.CheckAssignToBlank,
		ExcludeFunctions:         old.ExcludeFunctions,
	}
}

func toErrChkJSONSettings(old versionone.ErrChkJSONSettings) versiontwo.ErrChkJSONSettings {
	return versiontwo.ErrChkJSONSettings{
		CheckErrorFreeEncoding: old.CheckErrorFreeEncoding,
		ReportNoExported:       old.ReportNoExported,
	}
}

func toErrorLintSettings(old versionone.ErrorLintSettings) versiontwo.ErrorLintSettings {
	settings := versiontwo.ErrorLintSettings{
		Errorf:      old.Errorf,
		ErrorfMulti: old.ErrorfMulti,
		Asserts:     old.Asserts,
		Comparison:  old.Comparison,
	}

	for _, allowedError := range old.AllowedErrors {
		settings.AllowedErrors = append(settings.AllowedErrors, versiontwo.ErrorLintAllowPair{
			Err: allowedError.Err,
			Fun: allowedError.Fun,
		})
	}
	for _, allowedError := range old.AllowedErrorsWildcard {
		settings.AllowedErrorsWildcard = append(settings.AllowedErrorsWildcard, versiontwo.ErrorLintAllowPair{
			Err: allowedError.Err,
			Fun: allowedError.Fun,
		})
	}

	return settings
}

func toExhaustiveSettings(old versionone.ExhaustiveSettings) versiontwo.ExhaustiveSettings {
	return versiontwo.ExhaustiveSettings{
		Check:                      old.Check,
		DefaultSignifiesExhaustive: old.DefaultSignifiesExhaustive,
		IgnoreEnumMembers:          old.IgnoreEnumMembers,
		IgnoreEnumTypes:            old.IgnoreEnumTypes,
		PackageScopeOnly:           old.PackageScopeOnly,
		ExplicitExhaustiveMap:      old.ExplicitExhaustiveMap,
		ExplicitExhaustiveSwitch:   old.ExplicitExhaustiveSwitch,
		DefaultCaseRequired:        old.DefaultCaseRequired,
	}
}

func toExhaustructSettings(old versionone.ExhaustructSettings) versiontwo.ExhaustructSettings {
	return versiontwo.ExhaustructSettings{
		Include: old.Include,
		Exclude: old.Exclude,
	}
}

func toFatcontextSettings(old versionone.FatcontextSettings) versiontwo.FatcontextSettings {
	return versiontwo.FatcontextSettings{
		CheckStructPointers: old.CheckStructPointers,
	}
}

func toForbidigoSettings(old versionone.ForbidigoSettings) versiontwo.ForbidigoSettings {
	settings := versiontwo.ForbidigoSettings{
		ExcludeGodocExamples: old.ExcludeGodocExamples,
		AnalyzeTypes:         old.AnalyzeTypes,
	}

	for _, pattern := range old.Forbid {
		if pattern.Pattern == nil && pattern.Msg == nil && pattern.Package == nil {
			buffer, err := pattern.MarshalString()
			if err != nil {
				// impossible case
				panic(err)
			}

			settings.Forbid = append(settings.Forbid, versiontwo.ForbidigoPattern{
				Pattern: ptr.Pointer(string(buffer)),
			})

			continue
		}

		settings.Forbid = append(settings.Forbid, versiontwo.ForbidigoPattern{
			Pattern: pattern.Pattern,
			Package: pattern.Package,
			Msg:     pattern.Msg,
		})
	}

	return settings
}

func toFunlenSettings(old versionone.FunlenSettings) versiontwo.FunlenSettings {
	return versiontwo.FunlenSettings{
		Lines:          old.Lines,
		Statements:     old.Statements,
		IgnoreComments: old.IgnoreComments,
	}
}

func toGinkgoLinterSettings(old versionone.GinkgoLinterSettings) versiontwo.GinkgoLinterSettings {
	return versiontwo.GinkgoLinterSettings{
		SuppressLenAssertion:       old.SuppressLenAssertion,
		SuppressNilAssertion:       old.SuppressNilAssertion,
		SuppressErrAssertion:       old.SuppressErrAssertion,
		SuppressCompareAssertion:   old.SuppressCompareAssertion,
		SuppressAsyncAssertion:     old.SuppressAsyncAssertion,
		SuppressTypeCompareWarning: old.SuppressTypeCompareWarning,
		ForbidFocusContainer:       old.ForbidFocusContainer,
		AllowHaveLenZero:           old.AllowHaveLenZero,
		ForceExpectTo:              old.ForceExpectTo,
		ValidateAsyncIntervals:     old.ValidateAsyncIntervals,
		ForbidSpecPollution:        old.ForbidSpecPollution,
		ForceSucceedForFuncs:       old.ForceSucceedForFuncs,
	}
}

func toGocognitSettings(old versionone.GocognitSettings) versiontwo.GocognitSettings {
	return versiontwo.GocognitSettings{
		MinComplexity: old.MinComplexity,
	}
}

func toGoChecksumTypeSettings(old versionone.GoChecksumTypeSettings) versiontwo.GoChecksumTypeSettings {
	return versiontwo.GoChecksumTypeSettings{
		DefaultSignifiesExhaustive: old.DefaultSignifiesExhaustive,
		IncludeSharedInterfaces:    old.IncludeSharedInterfaces,
	}
}

func toGoConstSettings(old versionone.GoConstSettings) versiontwo.GoConstSettings {
	return versiontwo.GoConstSettings{
		IgnoreStrings:       old.IgnoreStrings,
		MatchWithConstants:  old.MatchWithConstants,
		MinStringLen:        old.MinStringLen,
		MinOccurrencesCount: old.MinOccurrencesCount,
		ParseNumbers:        old.ParseNumbers,
		NumberMin:           old.NumberMin,
		NumberMax:           old.NumberMax,
		IgnoreCalls:         old.IgnoreCalls,
	}
}

func toGoCriticSettings(old versionone.GoCriticSettings) versiontwo.GoCriticSettings {
	settings := versiontwo.GoCriticSettings{
		Go:             old.Go,
		DisableAll:     old.DisableAll,
		EnabledChecks:  old.EnabledChecks,
		EnableAll:      old.EnableAll,
		DisabledChecks: old.DisabledChecks,
		EnabledTags:    old.EnabledTags,
		DisabledTags:   old.DisabledTags,
	}

	for k, checkSettings := range old.SettingsPerCheck {
		if settings.SettingsPerCheck == nil {
			settings.SettingsPerCheck = make(map[string]versiontwo.GoCriticCheckSettings)
		}

		if k != "ruleguard" {
			settings.SettingsPerCheck[k] = versiontwo.GoCriticCheckSettings(checkSettings)

			continue
		}

		gccs := versiontwo.GoCriticCheckSettings{}

		for sk, value := range checkSettings {
			if sk != "rules" {
				gccs[sk] = value

				continue
			}

			if rules, ok := value.(string); ok {
				gccs[sk] = strings.ReplaceAll(rules, "${configDir}", "${base-path}")
			}
		}

		settings.SettingsPerCheck[k] = gccs
	}

	return settings
}

func toGoCycloSettings(old versionone.GoCycloSettings) versiontwo.GoCycloSettings {
	return versiontwo.GoCycloSettings{
		MinComplexity: old.MinComplexity,
	}
}

func toGodotSettings(old versionone.GodotSettings) versiontwo.GodotSettings {
	return versiontwo.GodotSettings{
		Scope:   old.Scope,
		Exclude: old.Exclude,
		Capital: old.Capital,
		Period:  old.Period,
	}
}

func toGodoxSettings(old versionone.GodoxSettings) versiontwo.GodoxSettings {
	return versiontwo.GodoxSettings{
		Keywords: old.Keywords,
	}
}

func toGoHeaderSettings(old versionone.GoHeaderSettings) versiontwo.GoHeaderSettings {
	return versiontwo.GoHeaderSettings{
		Values:       old.Values,
		Template:     old.Template,
		TemplatePath: old.TemplatePath,
	}
}

func toGoModDirectivesSettings(old versionone.GoModDirectivesSettings) versiontwo.GoModDirectivesSettings {
	return versiontwo.GoModDirectivesSettings{
		ReplaceAllowList:          old.ReplaceAllowList,
		ReplaceLocal:              old.ReplaceLocal,
		ExcludeForbidden:          old.ExcludeForbidden,
		RetractAllowNoExplanation: old.RetractAllowNoExplanation,
		ToolchainForbidden:        old.ToolchainForbidden,
		ToolchainPattern:          old.ToolchainPattern,
		ToolForbidden:             old.ToolForbidden,
		GoDebugForbidden:          old.GoDebugForbidden,
		GoVersionPattern:          old.GoVersionPattern,
	}
}

func toGoModGuardSettings(old versionone.GoModGuardSettings) versiontwo.GoModGuardSettings {
	blocked := versiontwo.GoModGuardBlocked{
		LocalReplaceDirectives: old.Blocked.LocalReplaceDirectives,
	}

	for _, version := range old.Blocked.Modules {
		data := map[string]versiontwo.GoModGuardModule{}

		for k, v := range version {
			data[k] = versiontwo.GoModGuardModule{
				Recommendations: v.Recommendations,
				Reason:          v.Reason,
			}
		}

		blocked.Modules = append(blocked.Modules, data)
	}

	for _, version := range old.Blocked.Versions {
		data := map[string]versiontwo.GoModGuardVersion{}

		for k, v := range version {
			data[k] = versiontwo.GoModGuardVersion{
				Version: v.Version,
				Reason:  v.Reason,
			}
		}

		blocked.Versions = append(blocked.Versions, data)
	}

	return versiontwo.GoModGuardSettings{
		Allowed: versiontwo.GoModGuardAllowed{
			Modules: old.Allowed.Modules,
			Domains: old.Allowed.Domains,
		},
		Blocked: blocked,
	}
}

func toGoSecSettings(old versionone.GoSecSettings) versiontwo.GoSecSettings {
	return versiontwo.GoSecSettings{
		Includes:    old.Includes,
		Excludes:    old.Excludes,
		Severity:    old.Severity,
		Confidence:  old.Confidence,
		Config:      old.Config,
		Concurrency: old.Concurrency,
	}
}

func toGosmopolitanSettings(old versionone.GosmopolitanSettings) versiontwo.GosmopolitanSettings {
	return versiontwo.GosmopolitanSettings{
		AllowTimeLocal:  old.AllowTimeLocal,
		EscapeHatches:   old.EscapeHatches,
		WatchForScripts: old.WatchForScripts,
	}
}

func toGovetSettings(old versionone.GovetSettings) versiontwo.GovetSettings {
	return versiontwo.GovetSettings{
		Go:         old.Go,
		Enable:     old.Enable,
		Disable:    old.Disable,
		EnableAll:  old.EnableAll,
		DisableAll: old.DisableAll,
		Settings:   old.Settings,
	}
}

func toGrouperSettings(old versionone.GrouperSettings) versiontwo.GrouperSettings {
	return versiontwo.GrouperSettings{
		ConstRequireSingleConst:   old.ConstRequireSingleConst,
		ConstRequireGrouping:      old.ConstRequireGrouping,
		ImportRequireSingleImport: old.ImportRequireSingleImport,
		ImportRequireGrouping:     old.ImportRequireGrouping,
		TypeRequireSingleType:     old.TypeRequireSingleType,
		TypeRequireGrouping:       old.TypeRequireGrouping,
		VarRequireSingleVar:       old.VarRequireSingleVar,
		VarRequireGrouping:        old.VarRequireGrouping,
	}
}

func toIfaceSettings(old versionone.IfaceSettings) versiontwo.IfaceSettings {
	return versiontwo.IfaceSettings{
		Enable:   old.Enable,
		Settings: old.Settings,
	}
}

func toImportAsSettings(old versionone.ImportAsSettings) versiontwo.ImportAsSettings {
	settings := versiontwo.ImportAsSettings{
		NoUnaliased:    old.NoUnaliased,
		NoExtraAliases: old.NoExtraAliases,
	}

	for _, alias := range old.Alias {
		settings.Alias = append(settings.Alias, versiontwo.ImportAsAlias{
			Pkg:   alias.Pkg,
			Alias: alias.Alias,
		})
	}

	return settings
}

func toINamedParamSettings(old versionone.INamedParamSettings) versiontwo.INamedParamSettings {
	return versiontwo.INamedParamSettings{
		SkipSingleParam: old.SkipSingleParam,
	}
}

func toInterfaceBloatSettings(old versionone.InterfaceBloatSettings) versiontwo.InterfaceBloatSettings {
	return versiontwo.InterfaceBloatSettings{
		Max: old.Max,
	}
}

func toIreturnSettings(old versionone.IreturnSettings) versiontwo.IreturnSettings {
	return versiontwo.IreturnSettings{
		Allow:  old.Allow,
		Reject: old.Reject,
	}
}

func toLllSettings(old versionone.LllSettings) versiontwo.LllSettings {
	return versiontwo.LllSettings{
		LineLength: old.LineLength,
		TabWidth:   old.TabWidth,
	}
}

func toLoggerCheckSettings(old versionone.LoggerCheckSettings) versiontwo.LoggerCheckSettings {
	return versiontwo.LoggerCheckSettings{
		Kitlog:           old.Kitlog,
		Klog:             old.Klog,
		Logr:             old.Logr,
		Slog:             old.Slog,
		Zap:              old.Zap,
		RequireStringKey: old.RequireStringKey,
		NoPrintfLike:     old.NoPrintfLike,
		Rules:            old.Rules,
	}
}

func toMaintIdxSettings(old versionone.MaintIdxSettings) versiontwo.MaintIdxSettings {
	return versiontwo.MaintIdxSettings{
		Under: old.Under,
	}
}

func toMakezeroSettings(old versionone.MakezeroSettings) versiontwo.MakezeroSettings {
	return versiontwo.MakezeroSettings{
		Always: old.Always,
	}
}

func toMisspellSettings(old versionone.MisspellSettings) versiontwo.MisspellSettings {
	settings := versiontwo.MisspellSettings{
		Mode:        old.Mode,
		Locale:      old.Locale,
		IgnoreRules: old.IgnoreWords,
	}

	for _, word := range old.ExtraWords {
		settings.ExtraWords = append(settings.ExtraWords, versiontwo.MisspellExtraWords{
			Typo:       word.Typo,
			Correction: word.Correction,
		})
	}

	return settings
}

func toMndSettings(old versionone.MndSettings) versiontwo.MndSettings {
	return versiontwo.MndSettings{
		Checks:           old.Checks,
		IgnoredNumbers:   old.IgnoredNumbers,
		IgnoredFiles:     old.IgnoredFiles,
		IgnoredFunctions: old.IgnoredFunctions,
	}
}

func toMustTagSettings(old versionone.MustTagSettings) versiontwo.MustTagSettings {
	settings := versiontwo.MustTagSettings{}

	for _, function := range old.Functions {
		settings.Functions = append(settings.Functions, versiontwo.MustTagFunction{
			Name:   function.Name,
			Tag:    function.Tag,
			ArgPos: function.ArgPos,
		})
	}

	return settings
}

func toNakedretSettings(old versionone.NakedretSettings) versiontwo.NakedretSettings {
	return versiontwo.NakedretSettings{
		MaxFuncLines: old.MaxFuncLines,
	}
}

func toNestifSettings(old versionone.NestifSettings) versiontwo.NestifSettings {
	return versiontwo.NestifSettings{
		MinComplexity: old.MinComplexity,
	}
}

func toNilNilSettings(old versionone.NilNilSettings) versiontwo.NilNilSettings {
	return versiontwo.NilNilSettings{
		DetectOpposite: old.DetectOpposite,
		CheckedTypes:   old.CheckedTypes,
	}
}

func toNlreturnSettings(old versionone.NlreturnSettings) versiontwo.NlreturnSettings {
	return versiontwo.NlreturnSettings{
		BlockSize: old.BlockSize,
	}
}

func toNoLintLintSettings(old versionone.NoLintLintSettings) versiontwo.NoLintLintSettings {
	return versiontwo.NoLintLintSettings{
		RequireExplanation: old.RequireExplanation,
		RequireSpecific:    old.RequireSpecific,
		AllowNoExplanation: old.AllowNoExplanation,
		AllowUnused:        old.AllowUnused,
	}
}

func toNoNamedReturnsSettings(old versionone.NoNamedReturnsSettings) versiontwo.NoNamedReturnsSettings {
	return versiontwo.NoNamedReturnsSettings{
		ReportErrorInDefer: old.ReportErrorInDefer,
	}
}

func toParallelTestSettings(old versionone.ParallelTestSettings) versiontwo.ParallelTestSettings {
	return versiontwo.ParallelTestSettings{
		Go:                    nil,
		IgnoreMissing:         old.IgnoreMissing,
		IgnoreMissingSubtests: old.IgnoreMissingSubtests,
	}
}

func toPerfSprintSettings(old versionone.PerfSprintSettings) versiontwo.PerfSprintSettings {
	return versiontwo.PerfSprintSettings{
		IntegerFormat: old.IntegerFormat,
		IntConversion: old.IntConversion,
		ErrorFormat:   old.ErrorFormat,
		ErrError:      old.ErrError,
		ErrorF:        old.ErrorF,
		StringFormat:  old.StringFormat,
		SprintF1:      old.SprintF1,
		StrConcat:     old.StrConcat,
		BoolFormat:    old.BoolFormat,
		HexFormat:     old.HexFormat,
	}
}

func toPreallocSettings(old versionone.PreallocSettings) versiontwo.PreallocSettings {
	return versiontwo.PreallocSettings{
		Simple:     old.Simple,
		RangeLoops: old.RangeLoops,
		ForLoops:   old.ForLoops,
	}
}

func toPredeclaredSettings(old versionone.PredeclaredSettings) versiontwo.PredeclaredSettings {
	var ignore []string
	if ptr.Deref(old.Ignore) != "" {
		ignore = strings.Split(ptr.Deref(old.Ignore), ",")
	}

	return versiontwo.PredeclaredSettings{
		Ignore:    ignore,
		Qualified: old.Qualified,
	}
}

func toPromlinterSettings(old versionone.PromlinterSettings) versiontwo.PromlinterSettings {
	return versiontwo.PromlinterSettings{
		Strict:          old.Strict,
		DisabledLinters: old.DisabledLinters,
	}
}

func toProtoGetterSettings(old versionone.ProtoGetterSettings) versiontwo.ProtoGetterSettings {
	return versiontwo.ProtoGetterSettings{
		SkipGeneratedBy:         old.SkipGeneratedBy,
		SkipFiles:               old.SkipFiles,
		SkipAnyGenerated:        old.SkipAnyGenerated,
		ReplaceFirstArgInAppend: old.ReplaceFirstArgInAppend,
	}
}

func toReassignSettings(old versionone.ReassignSettings) versiontwo.ReassignSettings {
	return versiontwo.ReassignSettings{
		Patterns: old.Patterns,
	}
}

func toRecvcheckSettings(old versionone.RecvcheckSettings) versiontwo.RecvcheckSettings {
	return versiontwo.RecvcheckSettings{
		DisableBuiltin: old.DisableBuiltin,
		Exclusions:     old.Exclusions,
	}
}

func toReviveSettings(old versionone.ReviveSettings) versiontwo.ReviveSettings {
	settings := versiontwo.ReviveSettings{
		MaxOpenFiles:   old.MaxOpenFiles,
		Confidence:     old.Confidence,
		Severity:       old.Severity,
		EnableAllRules: old.EnableAllRules,
		ErrorCode:      old.ErrorCode,
		WarningCode:    old.WarningCode,
	}

	for _, rule := range old.Rules {
		settings.Rules = append(settings.Rules, versiontwo.ReviveRule{
			Name:      rule.Name,
			Arguments: rule.Arguments,
			Severity:  rule.Severity,
			Disabled:  rule.Disabled,
			Exclude:   rule.Exclude,
		})
	}

	for _, directive := range old.Directives {
		settings.Directives = append(settings.Directives, versiontwo.ReviveDirective{
			Name:     directive.Name,
			Severity: directive.Severity,
		})
	}

	return settings
}

func toRowsErrCheckSettings(old versionone.RowsErrCheckSettings) versiontwo.RowsErrCheckSettings {
	return versiontwo.RowsErrCheckSettings{
		Packages: old.Packages,
	}
}

func toSlogLintSettings(old versionone.SlogLintSettings) versiontwo.SlogLintSettings {
	return versiontwo.SlogLintSettings{
		NoMixedArgs:    old.NoMixedArgs,
		KVOnly:         old.KVOnly,
		AttrOnly:       old.AttrOnly,
		NoGlobal:       old.NoGlobal,
		Context:        old.Context,
		StaticMsg:      old.StaticMsg,
		NoRawKeys:      old.NoRawKeys,
		KeyNamingCase:  old.KeyNamingCase,
		ForbiddenKeys:  old.ForbiddenKeys,
		ArgsOnSepLines: old.ArgsOnSepLines,
	}
}

func toSpancheckSettings(old versionone.SpancheckSettings) versiontwo.SpancheckSettings {
	return versiontwo.SpancheckSettings{
		Checks:                   old.Checks,
		IgnoreCheckSignatures:    old.IgnoreCheckSignatures,
		ExtraStartSpanSignatures: old.ExtraStartSpanSignatures,
	}
}

func toStaticCheckSettings(old versionone.LintersSettings) versiontwo.StaticCheckSettings {
	var checks []string

	for _, check := range slices.Concat(old.Staticcheck.Checks, old.Stylecheck.Checks, old.Gosimple.Checks) {
		if check == "*" {
			checks = append(checks, "all")
			continue
		}
		checks = append(checks, check)
	}

	checks = Unique(checks)

	slices.SortFunc(checks, func(a, b string) int {
		if a == "all" {
			return -1
		}

		if b == "all" {
			return 1
		}

		return strings.Compare(a, b)
	})

	return versiontwo.StaticCheckSettings{
		Checks:                  checks,
		Initialisms:             old.Stylecheck.Initialisms,
		DotImportWhitelist:      old.Stylecheck.DotImportWhitelist,
		HTTPStatusCodeWhitelist: old.Stylecheck.HTTPStatusCodeWhitelist,
	}
}

func toTagAlignSettings(old versionone.TagAlignSettings) versiontwo.TagAlignSettings {
	return versiontwo.TagAlignSettings{
		Align:  old.Align,
		Sort:   old.Sort,
		Order:  old.Order,
		Strict: old.Strict,
	}
}

func toTagliatelleSettings(old versionone.TagliatelleSettings) versiontwo.TagliatelleSettings {
	tcase := versiontwo.TagliatelleCase{
		TagliatelleBase: versiontwo.TagliatelleBase{
			Rules:         old.Case.Rules,
			UseFieldName:  old.Case.UseFieldName,
			IgnoredFields: old.Case.IgnoredFields,
		},
		Overrides: []versiontwo.TagliatelleOverrides{},
	}

	for k, rule := range old.Case.ExtendedRules {
		if tcase.ExtendedRules == nil {
			tcase.ExtendedRules = make(map[string]versiontwo.TagliatelleExtendedRule)
		}

		tcase.ExtendedRules[k] = versiontwo.TagliatelleExtendedRule{
			Case:                rule.Case,
			ExtraInitialisms:    rule.ExtraInitialisms,
			InitialismOverrides: rule.InitialismOverrides,
		}
	}

	return versiontwo.TagliatelleSettings{Case: tcase}
}

func toTestifylintSettings(old versionone.TestifylintSettings) versiontwo.TestifylintSettings {
	return versiontwo.TestifylintSettings{
		EnableAll:        old.EnableAll,
		DisableAll:       old.DisableAll,
		EnabledCheckers:  old.EnabledCheckers,
		DisabledCheckers: old.DisabledCheckers,
		BoolCompare: versiontwo.TestifylintBoolCompare{
			IgnoreCustomTypes: old.BoolCompare.IgnoreCustomTypes,
		},
		ExpectedActual: versiontwo.TestifylintExpectedActual{
			ExpVarPattern: old.ExpectedActual.ExpVarPattern,
		},
		Formatter: versiontwo.TestifylintFormatter{
			CheckFormatString: old.Formatter.CheckFormatString,
			RequireFFuncs:     old.Formatter.RequireFFuncs,
		},
		GoRequire: versiontwo.TestifylintGoRequire{
			IgnoreHTTPHandlers: old.GoRequire.IgnoreHTTPHandlers,
		},
		RequireError: versiontwo.TestifylintRequireError{
			FnPattern: old.RequireError.FnPattern,
		},
		SuiteExtraAssertCall: versiontwo.TestifylintSuiteExtraAssertCall{
			Mode: old.SuiteExtraAssertCall.Mode,
		},
	}
}

func toTestpackageSettings(old versionone.TestpackageSettings) versiontwo.TestpackageSettings {
	return versiontwo.TestpackageSettings{
		SkipRegexp:    old.SkipRegexp,
		AllowPackages: old.AllowPackages,
	}
}

func toThelperSettings(old versionone.ThelperSettings) versiontwo.ThelperSettings {
	return versiontwo.ThelperSettings{
		Test: versiontwo.ThelperOptions{
			First: old.Test.First,
			Name:  old.Test.Name,
			Begin: old.Test.Begin,
		},
		Fuzz: versiontwo.ThelperOptions{
			First: old.Fuzz.First,
			Name:  old.Fuzz.Name,
			Begin: old.Fuzz.Begin,
		},
		Benchmark: versiontwo.ThelperOptions{
			First: old.Benchmark.First,
			Name:  old.Benchmark.Name,
			Begin: old.Benchmark.Begin,
		},
		TB: versiontwo.ThelperOptions{
			First: old.TB.First,
			Name:  old.TB.Name,
			Begin: old.TB.Begin,
		},
	}
}

func toUnconvertSettings(old versionone.UnconvertSettings) versiontwo.UnconvertSettings {
	return versiontwo.UnconvertSettings{
		FastMath: old.FastMath,
		Safe:     old.Safe,
	}
}

func toUnparamSettings(old versionone.UnparamSettings) versiontwo.UnparamSettings {
	return versiontwo.UnparamSettings{
		CheckExported: old.CheckExported,
	}
}

func toUnusedSettings(old versionone.UnusedSettings) versiontwo.UnusedSettings {
	return versiontwo.UnusedSettings{
		FieldWritesAreUses:     old.FieldWritesAreUses,
		PostStatementsAreReads: old.PostStatementsAreReads,
		ExportedFieldsAreUsed:  old.ExportedFieldsAreUsed,
		ParametersAreUsed:      old.ParametersAreUsed,
		LocalVariablesAreUsed:  old.LocalVariablesAreUsed,
		GeneratedIsUsed:        old.GeneratedIsUsed,
	}
}

func toUseStdlibVarsSettings(old versionone.UseStdlibVarsSettings) versiontwo.UseStdlibVarsSettings {
	return versiontwo.UseStdlibVarsSettings{
		HTTPMethod:         old.HTTPMethod,
		HTTPStatusCode:     old.HTTPStatusCode,
		TimeWeekday:        old.TimeWeekday,
		TimeMonth:          old.TimeMonth,
		TimeLayout:         old.TimeLayout,
		CryptoHash:         old.CryptoHash,
		DefaultRPCPath:     old.DefaultRPCPath,
		SQLIsolationLevel:  old.SQLIsolationLevel,
		TLSSignatureScheme: old.TLSSignatureScheme,
		ConstantKind:       old.ConstantKind,
	}
}

func toUseTestingSettings(old versionone.UseTestingSettings) versiontwo.UseTestingSettings {
	return versiontwo.UseTestingSettings{
		ContextBackground: old.ContextBackground,
		ContextTodo:       old.ContextTodo,
		OSChdir:           old.OSChdir,
		OSMkdirTemp:       old.OSMkdirTemp,
		OSSetenv:          old.OSSetenv,
		OSTempDir:         old.OSTempDir,
		OSCreateTemp:      old.OSCreateTemp,
	}
}

func toVarnamelenSettings(old versionone.VarnamelenSettings) versiontwo.VarnamelenSettings {
	return versiontwo.VarnamelenSettings{
		MaxDistance:        old.MaxDistance,
		MinNameLength:      old.MinNameLength,
		CheckReceiver:      old.CheckReceiver,
		CheckReturn:        old.CheckReturn,
		CheckTypeParam:     old.CheckTypeParam,
		IgnoreNames:        old.IgnoreNames,
		IgnoreTypeAssertOk: old.IgnoreTypeAssertOk,
		IgnoreMapIndexOk:   old.IgnoreMapIndexOk,
		IgnoreChanRecvOk:   old.IgnoreChanRecvOk,
		IgnoreDecls:        old.IgnoreDecls,
	}
}

func toWhitespaceSettings(old versionone.WhitespaceSettings) versiontwo.WhitespaceSettings {
	return versiontwo.WhitespaceSettings{
		MultiIf:   old.MultiIf,
		MultiFunc: old.MultiFunc,
	}
}

func toWrapcheckSettings(old versionone.WrapcheckSettings) versiontwo.WrapcheckSettings {
	return versiontwo.WrapcheckSettings{
		ExtraIgnoreSigs:        old.ExtraIgnoreSigs,
		IgnoreSigs:             old.IgnoreSigs,
		IgnoreSigRegexps:       old.IgnoreSigRegexps,
		IgnorePackageGlobs:     old.IgnorePackageGlobs,
		IgnoreInterfaceRegexps: old.IgnoreInterfaceRegexps,
	}
}

func toWSLSettings(old versionone.WSLSettings) versiontwo.WSLv4Settings {
	return versiontwo.WSLv4Settings{
		StrictAppend:                     old.StrictAppend,
		AllowAssignAndCallCuddle:         old.AllowAssignAndCallCuddle,
		AllowAssignAndAnythingCuddle:     old.AllowAssignAndAnythingCuddle,
		AllowMultiLineAssignCuddle:       old.AllowMultiLineAssignCuddle,
		ForceCaseTrailingWhitespaceLimit: old.ForceCaseTrailingWhitespaceLimit,
		AllowTrailingComment:             old.AllowTrailingComment,
		AllowSeparatedLeadingComment:     old.AllowSeparatedLeadingComment,
		AllowCuddleDeclaration:           old.AllowCuddleDeclaration,
		AllowCuddleWithCalls:             old.AllowCuddleWithCalls,
		AllowCuddleWithRHS:               old.AllowCuddleWithRHS,
		ForceCuddleErrCheckAndAssign:     old.ForceCuddleErrCheckAndAssign,
		ErrorVariableNames:               old.ErrorVariableNames,
		ForceExclusiveShortDeclarations:  old.ForceExclusiveShortDeclarations,
	}
}

func toCustom(old map[string]versionone.CustomLinterSettings) map[string]versiontwo.CustomLinterSettings {
	if old == nil {
		return nil
	}

	settings := map[string]versiontwo.CustomLinterSettings{}

	for k, s := range old {
		settings[k] = versiontwo.CustomLinterSettings{
			Type:        s.Type,
			Path:        s.Path,
			Description: s.Description,
			OriginalURL: s.OriginalURL,
			Settings:    s.Settings,
		}
	}

	return settings
}
