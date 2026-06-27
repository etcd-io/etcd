package versionone

import (
	"encoding"

	"go.yaml.in/yaml/v3"

	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/ptr"
)

type LintersSettings struct {
	Asasalint       AsasalintSettings       `mapstructure:"asasalint"`
	BiDiChk         BiDiChkSettings         `mapstructure:"bidichk"`
	CopyLoopVar     CopyLoopVarSettings     `mapstructure:"copyloopvar"`
	Cyclop          Cyclop                  `mapstructure:"cyclop"`
	Decorder        DecorderSettings        `mapstructure:"decorder"`
	Depguard        DepGuardSettings        `mapstructure:"depguard"`
	Dogsled         DogsledSettings         `mapstructure:"dogsled"`
	Dupl            DuplSettings            `mapstructure:"dupl"`
	DupWord         DupWordSettings         `mapstructure:"dupword"`
	Errcheck        ErrcheckSettings        `mapstructure:"errcheck"`
	ErrChkJSON      ErrChkJSONSettings      `mapstructure:"errchkjson"`
	ErrorLint       ErrorLintSettings       `mapstructure:"errorlint"`
	Exhaustive      ExhaustiveSettings      `mapstructure:"exhaustive"`
	Exhaustruct     ExhaustructSettings     `mapstructure:"exhaustruct"`
	Fatcontext      FatcontextSettings      `mapstructure:"fatcontext"`
	Forbidigo       ForbidigoSettings       `mapstructure:"forbidigo"`
	Funlen          FunlenSettings          `mapstructure:"funlen"`
	GinkgoLinter    GinkgoLinterSettings    `mapstructure:"ginkgolinter"`
	Gocognit        GocognitSettings        `mapstructure:"gocognit"`
	GoChecksumType  GoChecksumTypeSettings  `mapstructure:"gochecksumtype"`
	Goconst         GoConstSettings         `mapstructure:"goconst"`
	Gocritic        GoCriticSettings        `mapstructure:"gocritic"`
	Gocyclo         GoCycloSettings         `mapstructure:"gocyclo"`
	Godot           GodotSettings           `mapstructure:"godot"`
	Godox           GodoxSettings           `mapstructure:"godox"`
	Goheader        GoHeaderSettings        `mapstructure:"goheader"`
	GoModDirectives GoModDirectivesSettings `mapstructure:"gomoddirectives"`
	Gomodguard      GoModGuardSettings      `mapstructure:"gomodguard"`
	Gosec           GoSecSettings           `mapstructure:"gosec"`
	Gosimple        StaticCheckSettings     `mapstructure:"gosimple"`
	Gosmopolitan    GosmopolitanSettings    `mapstructure:"gosmopolitan"`
	Govet           GovetSettings           `mapstructure:"govet"`
	Grouper         GrouperSettings         `mapstructure:"grouper"`
	Iface           IfaceSettings           `mapstructure:"iface"`
	ImportAs        ImportAsSettings        `mapstructure:"importas"`
	Inamedparam     INamedParamSettings     `mapstructure:"inamedparam"`
	InterfaceBloat  InterfaceBloatSettings  `mapstructure:"interfacebloat"`
	Ireturn         IreturnSettings         `mapstructure:"ireturn"`
	Lll             LllSettings             `mapstructure:"lll"`
	LoggerCheck     LoggerCheckSettings     `mapstructure:"loggercheck"`
	MaintIdx        MaintIdxSettings        `mapstructure:"maintidx"`
	Makezero        MakezeroSettings        `mapstructure:"makezero"`
	Misspell        MisspellSettings        `mapstructure:"misspell"`
	Mnd             MndSettings             `mapstructure:"mnd"`
	MustTag         MustTagSettings         `mapstructure:"musttag"`
	Nakedret        NakedretSettings        `mapstructure:"nakedret"`
	Nestif          NestifSettings          `mapstructure:"nestif"`
	NilNil          NilNilSettings          `mapstructure:"nilnil"`
	Nlreturn        NlreturnSettings        `mapstructure:"nlreturn"`
	NoLintLint      NoLintLintSettings      `mapstructure:"nolintlint"`
	NoNamedReturns  NoNamedReturnsSettings  `mapstructure:"nonamedreturns"`
	ParallelTest    ParallelTestSettings    `mapstructure:"paralleltest"`
	PerfSprint      PerfSprintSettings      `mapstructure:"perfsprint"`
	Prealloc        PreallocSettings        `mapstructure:"prealloc"`
	Predeclared     PredeclaredSettings     `mapstructure:"predeclared"`
	Promlinter      PromlinterSettings      `mapstructure:"promlinter"`
	ProtoGetter     ProtoGetterSettings     `mapstructure:"protogetter"`
	Reassign        ReassignSettings        `mapstructure:"reassign"`
	Recvcheck       RecvcheckSettings       `mapstructure:"recvcheck"`
	Revive          ReviveSettings          `mapstructure:"revive"`
	RowsErrCheck    RowsErrCheckSettings    `mapstructure:"rowserrcheck"`
	SlogLint        SlogLintSettings        `mapstructure:"sloglint"`
	Spancheck       SpancheckSettings       `mapstructure:"spancheck"`
	Staticcheck     StaticCheckSettings     `mapstructure:"staticcheck"`
	Stylecheck      StaticCheckSettings     `mapstructure:"stylecheck"`
	TagAlign        TagAlignSettings        `mapstructure:"tagalign"`
	Tagliatelle     TagliatelleSettings     `mapstructure:"tagliatelle"`
	Tenv            TenvSettings            `mapstructure:"tenv"`
	Testifylint     TestifylintSettings     `mapstructure:"testifylint"`
	Testpackage     TestpackageSettings     `mapstructure:"testpackage"`
	Thelper         ThelperSettings         `mapstructure:"thelper"`
	Unconvert       UnconvertSettings       `mapstructure:"unconvert"`
	Unparam         UnparamSettings         `mapstructure:"unparam"`
	Unused          UnusedSettings          `mapstructure:"unused"`
	UseStdlibVars   UseStdlibVarsSettings   `mapstructure:"usestdlibvars"`
	UseTesting      UseTestingSettings      `mapstructure:"usetesting"`
	Varnamelen      VarnamelenSettings      `mapstructure:"varnamelen"`
	Whitespace      WhitespaceSettings      `mapstructure:"whitespace"`
	Wrapcheck       WrapcheckSettings       `mapstructure:"wrapcheck"`
	WSL             WSLSettings             `mapstructure:"wsl"`

	Custom map[string]CustomLinterSettings `mapstructure:"custom"`

	Gci       GciSettings       `mapstructure:"gci"`
	GoFmt     GoFmtSettings     `mapstructure:"gofmt"`
	GoFumpt   GoFumptSettings   `mapstructure:"gofumpt"`
	GoImports GoImportsSettings `mapstructure:"goimports"`
}

type AsasalintSettings struct {
	Exclude              []string `mapstructure:"exclude"`
	UseBuiltinExclusions *bool    `mapstructure:"use-builtin-exclusions"`
	IgnoreTest           *bool    `mapstructure:"ignore-test"`
}

type BiDiChkSettings struct {
	LeftToRightEmbedding     *bool `mapstructure:"left-to-right-embedding"`
	RightToLeftEmbedding     *bool `mapstructure:"right-to-left-embedding"`
	PopDirectionalFormatting *bool `mapstructure:"pop-directional-formatting"`
	LeftToRightOverride      *bool `mapstructure:"left-to-right-override"`
	RightToLeftOverride      *bool `mapstructure:"right-to-left-override"`
	LeftToRightIsolate       *bool `mapstructure:"left-to-right-isolate"`
	RightToLeftIsolate       *bool `mapstructure:"right-to-left-isolate"`
	FirstStrongIsolate       *bool `mapstructure:"first-strong-isolate"`
	PopDirectionalIsolate    *bool `mapstructure:"pop-directional-isolate"`
}

type CopyLoopVarSettings struct {
	CheckAlias *bool `mapstructure:"check-alias"`

	// Deprecated: use CheckAlias
	IgnoreAlias *bool `mapstructure:"ignore-alias"`
}

type Cyclop struct {
	MaxComplexity  *int     `mapstructure:"max-complexity"`
	PackageAverage *float64 `mapstructure:"package-average"`
	SkipTests      *bool    `mapstructure:"skip-tests"`
}

type DepGuardSettings struct {
	Rules map[string]*DepGuardList `mapstructure:"rules"`
}

type DepGuardList struct {
	ListMode *string        `mapstructure:"list-mode"`
	Files    []string       `mapstructure:"files"`
	Allow    []string       `mapstructure:"allow"`
	Deny     []DepGuardDeny `mapstructure:"deny"`
}

type DepGuardDeny struct {
	Pkg  *string `mapstructure:"pkg"`
	Desc *string `mapstructure:"desc"`
}

type DecorderSettings struct {
	DecOrder                  []string `mapstructure:"dec-order"`
	IgnoreUnderscoreVars      *bool    `mapstructure:"ignore-underscore-vars"`
	DisableDecNumCheck        *bool    `mapstructure:"disable-dec-num-check"`
	DisableTypeDecNumCheck    *bool    `mapstructure:"disable-type-dec-num-check"`
	DisableConstDecNumCheck   *bool    `mapstructure:"disable-const-dec-num-check"`
	DisableVarDecNumCheck     *bool    `mapstructure:"disable-var-dec-num-check"`
	DisableDecOrderCheck      *bool    `mapstructure:"disable-dec-order-check"`
	DisableInitFuncFirstCheck *bool    `mapstructure:"disable-init-func-first-check"`
}

type DogsledSettings struct {
	MaxBlankIdentifiers *int `mapstructure:"max-blank-identifiers"`
}

type DuplSettings struct {
	Threshold *int `mapstructure:"threshold"`
}

type DupWordSettings struct {
	Keywords []string `mapstructure:"keywords"`
	Ignore   []string `mapstructure:"ignore"`
}

type ErrcheckSettings struct {
	DisableDefaultExclusions *bool    `mapstructure:"disable-default-exclusions"`
	CheckTypeAssertions      *bool    `mapstructure:"check-type-assertions"`
	CheckAssignToBlank       *bool    `mapstructure:"check-blank"`
	ExcludeFunctions         []string `mapstructure:"exclude-functions"`

	// Deprecated: use ExcludeFunctions instead
	Exclude *string `mapstructure:"exclude"`

	// Deprecated: use ExcludeFunctions instead
	Ignore *string `mapstructure:"ignore"`
}

type ErrChkJSONSettings struct {
	CheckErrorFreeEncoding *bool `mapstructure:"check-error-free-encoding"`
	ReportNoExported       *bool `mapstructure:"report-no-exported"`
}

type ErrorLintSettings struct {
	Errorf                *bool                `mapstructure:"errorf"`
	ErrorfMulti           *bool                `mapstructure:"errorf-multi"`
	Asserts               *bool                `mapstructure:"asserts"`
	Comparison            *bool                `mapstructure:"comparison"`
	AllowedErrors         []ErrorLintAllowPair `mapstructure:"allowed-errors"`
	AllowedErrorsWildcard []ErrorLintAllowPair `mapstructure:"allowed-errors-wildcard"`
}

type ErrorLintAllowPair struct {
	Err *string `mapstructure:"err"`
	Fun *string `mapstructure:"fun"`
}

type ExhaustiveSettings struct {
	Check                      []string `mapstructure:"check"`
	CheckGenerated             *bool    `mapstructure:"check-generated"`
	DefaultSignifiesExhaustive *bool    `mapstructure:"default-signifies-exhaustive"`
	IgnoreEnumMembers          *string  `mapstructure:"ignore-enum-members"`
	IgnoreEnumTypes            *string  `mapstructure:"ignore-enum-types"`
	PackageScopeOnly           *bool    `mapstructure:"package-scope-only"`
	ExplicitExhaustiveMap      *bool    `mapstructure:"explicit-exhaustive-map"`
	ExplicitExhaustiveSwitch   *bool    `mapstructure:"explicit-exhaustive-switch"`
	DefaultCaseRequired        *bool    `mapstructure:"default-case-required"`
}

type ExhaustructSettings struct {
	Include []string `mapstructure:"include"`
	Exclude []string `mapstructure:"exclude"`
}

type FatcontextSettings struct {
	CheckStructPointers *bool `mapstructure:"check-struct-pointers"`
}

type ForbidigoSettings struct {
	Forbid               []ForbidigoPattern `mapstructure:"forbid"`
	ExcludeGodocExamples *bool              `mapstructure:"exclude-godoc-examples"`
	AnalyzeTypes         *bool              `mapstructure:"analyze-types"`
}

var _ encoding.TextUnmarshaler = &ForbidigoPattern{}

// ForbidigoPattern corresponds to forbidigo.pattern and adds mapstructure support.
// The YAML field names must match what forbidigo expects.
type ForbidigoPattern struct {
	// patternString gets populated when the config contains a *string as entry in ForbidigoSettings.Forbid[]
	// because ForbidigoPattern implements encoding.TextUnmarshaler
	// and the reader uses the mapstructure.TextUnmarshallerHookFunc as decoder hook.
	//
	// If the entry is a map, then the other fields are set as usual by mapstructure.
	patternString *string
	Pattern       *string `yaml:"p" mapstructure:"p"`
	Package       *string `yaml:"pkg,omitempty" mapstructure:"pkg,omitempty"`
	Msg           *string `yaml:"msg,omitempty" mapstructure:"msg,omitempty"`
}

func (p *ForbidigoPattern) UnmarshalText(text []byte) error {
	// Validation happens when instantiating forbidigo.
	p.patternString = ptr.Pointer(string(text))
	return nil
}

// MarshalString converts the pattern into a *string as needed by forbidigo.NewLinter.
//
// MarshalString is intentionally not called MarshalText,
// although it has the same signature
// because implementing encoding.TextMarshaler led to infinite recursion when yaml.Marshal called MarshalText.
func (p *ForbidigoPattern) MarshalString() ([]byte, error) {
	if ptr.Deref(p.patternString) != "" {
		return []byte(ptr.Deref(p.patternString)), nil
	}

	return yaml.Marshal(p)
}

type FunlenSettings struct {
	Lines          *int  `mapstructure:"lines"`
	Statements     *int  `mapstructure:"statements"`
	IgnoreComments *bool `mapstructure:"ignore-comments"`
}

type GinkgoLinterSettings struct {
	SuppressLenAssertion       *bool `mapstructure:"suppress-len-assertion"`
	SuppressNilAssertion       *bool `mapstructure:"suppress-nil-assertion"`
	SuppressErrAssertion       *bool `mapstructure:"suppress-err-assertion"`
	SuppressCompareAssertion   *bool `mapstructure:"suppress-compare-assertion"`
	SuppressAsyncAssertion     *bool `mapstructure:"suppress-async-assertion"`
	SuppressTypeCompareWarning *bool `mapstructure:"suppress-type-compare-assertion"`
	ForbidFocusContainer       *bool `mapstructure:"forbid-focus-container"`
	AllowHaveLenZero           *bool `mapstructure:"allow-havelen-zero"`
	ForceExpectTo              *bool `mapstructure:"force-expect-to"`
	ValidateAsyncIntervals     *bool `mapstructure:"validate-async-intervals"`
	ForbidSpecPollution        *bool `mapstructure:"forbid-spec-pollution"`
	ForceSucceedForFuncs       *bool `mapstructure:"force-succeed"`
}

type GoChecksumTypeSettings struct {
	DefaultSignifiesExhaustive *bool `mapstructure:"default-signifies-exhaustive"`
	IncludeSharedInterfaces    *bool `mapstructure:"include-shared-interfaces"`
}

type GocognitSettings struct {
	MinComplexity *int `mapstructure:"min-complexity"`
}

type GoConstSettings struct {
	IgnoreStrings       *string `mapstructure:"ignore-strings"`
	IgnoreTests         *bool   `mapstructure:"ignore-tests"`
	MatchWithConstants  *bool   `mapstructure:"match-constant"`
	MinStringLen        *int    `mapstructure:"min-len"`
	MinOccurrencesCount *int    `mapstructure:"min-occurrences"`
	ParseNumbers        *bool   `mapstructure:"numbers"`
	NumberMin           *int    `mapstructure:"min"`
	NumberMax           *int    `mapstructure:"max"`
	IgnoreCalls         *bool   `mapstructure:"ignore-calls"`
}

type GoCriticSettings struct {
	Go               *string                          `mapstructure:"-"`
	DisableAll       *bool                            `mapstructure:"disable-all"`
	EnabledChecks    []string                         `mapstructure:"enabled-checks"`
	EnableAll        *bool                            `mapstructure:"enable-all"`
	DisabledChecks   []string                         `mapstructure:"disabled-checks"`
	EnabledTags      []string                         `mapstructure:"enabled-tags"`
	DisabledTags     []string                         `mapstructure:"disabled-tags"`
	SettingsPerCheck map[string]GoCriticCheckSettings `mapstructure:"settings"`
}

type GoCriticCheckSettings map[string]any

type GoCycloSettings struct {
	MinComplexity *int `mapstructure:"min-complexity"`
}

type GodotSettings struct {
	Scope   *string  `mapstructure:"scope"`
	Exclude []string `mapstructure:"exclude"`
	Capital *bool    `mapstructure:"capital"`
	Period  *bool    `mapstructure:"period"`

	// Deprecated: use Scope instead
	CheckAll *bool `mapstructure:"check-all"`
}

type GodoxSettings struct {
	Keywords []string `mapstructure:"keywords"`
}

type GoHeaderSettings struct {
	Values       map[string]map[string]string `mapstructure:"values"`
	Template     *string                      `mapstructure:"template"`
	TemplatePath *string                      `mapstructure:"template-path"`
}

type GoModDirectivesSettings struct {
	ReplaceAllowList          []string `mapstructure:"replace-allow-list"`
	ReplaceLocal              *bool    `mapstructure:"replace-local"`
	ExcludeForbidden          *bool    `mapstructure:"exclude-forbidden"`
	RetractAllowNoExplanation *bool    `mapstructure:"retract-allow-no-explanation"`
	ToolchainForbidden        *bool    `mapstructure:"toolchain-forbidden"`
	ToolchainPattern          *string  `mapstructure:"toolchain-pattern"`
	ToolForbidden             *bool    `mapstructure:"tool-forbidden"`
	GoDebugForbidden          *bool    `mapstructure:"go-debug-forbidden"`
	GoVersionPattern          *string  `mapstructure:"go-version-pattern"`
}

type GoModGuardSettings struct {
	Allowed struct {
		Modules []string `mapstructure:"modules"`
		Domains []string `mapstructure:"domains"`
	} `mapstructure:"allowed"`
	Blocked struct {
		Modules []map[string]struct {
			Recommendations []string `mapstructure:"recommendations"`
			Reason          *string  `mapstructure:"reason"`
		} `mapstructure:"modules"`
		Versions []map[string]struct {
			Version *string `mapstructure:"version"`
			Reason  *string `mapstructure:"reason"`
		} `mapstructure:"versions"`
		LocalReplaceDirectives *bool `mapstructure:"local_replace_directives"`
	} `mapstructure:"blocked"`
}

type GoSecSettings struct {
	Includes         []string       `mapstructure:"includes"`
	Excludes         []string       `mapstructure:"excludes"`
	Severity         *string        `mapstructure:"severity"`
	Confidence       *string        `mapstructure:"confidence"`
	ExcludeGenerated *bool          `mapstructure:"exclude-generated"`
	Config           map[string]any `mapstructure:"config"`
	Concurrency      *int           `mapstructure:"concurrency"`
}

type GosmopolitanSettings struct {
	AllowTimeLocal  *bool    `mapstructure:"allow-time-local"`
	EscapeHatches   []string `mapstructure:"escape-hatches"`
	IgnoreTests     *bool    `mapstructure:"ignore-tests"`
	WatchForScripts []string `mapstructure:"watch-for-scripts"`
}

type GovetSettings struct {
	Go *string `mapstructure:"-"`

	Enable     []string `mapstructure:"enable"`
	Disable    []string `mapstructure:"disable"`
	EnableAll  *bool    `mapstructure:"enable-all"`
	DisableAll *bool    `mapstructure:"disable-all"`

	Settings map[string]map[string]any `mapstructure:"settings"`

	// Deprecated: the linter should be enabled inside Enable.
	CheckShadowing *bool `mapstructure:"check-shadowing"`
}

type GrouperSettings struct {
	ConstRequireSingleConst   *bool `mapstructure:"const-require-single-const"`
	ConstRequireGrouping      *bool `mapstructure:"const-require-grouping"`
	ImportRequireSingleImport *bool `mapstructure:"import-require-single-import"`
	ImportRequireGrouping     *bool `mapstructure:"import-require-grouping"`
	TypeRequireSingleType     *bool `mapstructure:"type-require-single-type"`
	TypeRequireGrouping       *bool `mapstructure:"type-require-grouping"`
	VarRequireSingleVar       *bool `mapstructure:"var-require-single-var"`
	VarRequireGrouping        *bool `mapstructure:"var-require-grouping"`
}

type IfaceSettings struct {
	Enable   []string                  `mapstructure:"enable"`
	Settings map[string]map[string]any `mapstructure:"settings"`
}

type ImportAsSettings struct {
	Alias          []ImportAsAlias `mapstructure:"alias"`
	NoUnaliased    *bool           `mapstructure:"no-unaliased"`
	NoExtraAliases *bool           `mapstructure:"no-extra-aliases"`
}

type ImportAsAlias struct {
	Pkg   *string `mapstructure:"pkg"`
	Alias *string `mapstructure:"alias"`
}

type INamedParamSettings struct {
	SkipSingleParam *bool `mapstructure:"skip-single-param"`
}

type InterfaceBloatSettings struct {
	Max *int `mapstructure:"max"`
}

type IreturnSettings struct {
	Allow  []string `mapstructure:"allow"`
	Reject []string `mapstructure:"reject"`
}

type LllSettings struct {
	LineLength *int `mapstructure:"line-length"`
	TabWidth   *int `mapstructure:"tab-width"`
}

type LoggerCheckSettings struct {
	Kitlog           *bool    `mapstructure:"kitlog"`
	Klog             *bool    `mapstructure:"klog"`
	Logr             *bool    `mapstructure:"logr"`
	Slog             *bool    `mapstructure:"slog"`
	Zap              *bool    `mapstructure:"zap"`
	RequireStringKey *bool    `mapstructure:"require-string-key"`
	NoPrintfLike     *bool    `mapstructure:"no-printf-like"`
	Rules            []string `mapstructure:"rules"`
}

type MaintIdxSettings struct {
	Under *int `mapstructure:"under"`
}

type MakezeroSettings struct {
	Always *bool `mapstructure:"always"`
}

type MisspellSettings struct {
	Mode       *string              `mapstructure:"mode"`
	Locale     *string              `mapstructure:"locale"`
	ExtraWords []MisspellExtraWords `mapstructure:"extra-words"`
	// TODO(ldez): v2 the option must be renamed to `IgnoredRules`.
	IgnoreWords []string `mapstructure:"ignore-words"`
}

type MisspellExtraWords struct {
	Typo       *string `mapstructure:"typo"`
	Correction *string `mapstructure:"correction"`
}

type MustTagSettings struct {
	Functions []struct {
		Name   *string `mapstructure:"name"`
		Tag    *string `mapstructure:"tag"`
		ArgPos *int    `mapstructure:"arg-pos"`
	} `mapstructure:"functions"`
}

type NakedretSettings struct {
	MaxFuncLines *uint `mapstructure:"max-func-lines"`
}

type NestifSettings struct {
	MinComplexity *int `mapstructure:"min-complexity"`
}

type NilNilSettings struct {
	DetectOpposite *bool    `mapstructure:"detect-opposite"`
	CheckedTypes   []string `mapstructure:"checked-types"`
}

type NlreturnSettings struct {
	BlockSize *int `mapstructure:"block-size"`
}

type MndSettings struct {
	Checks           []string `mapstructure:"checks"`
	IgnoredNumbers   []string `mapstructure:"ignored-numbers"`
	IgnoredFiles     []string `mapstructure:"ignored-files"`
	IgnoredFunctions []string `mapstructure:"ignored-functions"`
}

type NoLintLintSettings struct {
	RequireExplanation *bool    `mapstructure:"require-explanation"`
	RequireSpecific    *bool    `mapstructure:"require-specific"`
	AllowNoExplanation []string `mapstructure:"allow-no-explanation"`
	AllowUnused        *bool    `mapstructure:"allow-unused"`
}

type NoNamedReturnsSettings struct {
	ReportErrorInDefer *bool `mapstructure:"report-error-in-defer"`
}

type ParallelTestSettings struct {
	Go                    *string `mapstructure:"-"`
	IgnoreMissing         *bool   `mapstructure:"ignore-missing"`
	IgnoreMissingSubtests *bool   `mapstructure:"ignore-missing-subtests"`
}

type PerfSprintSettings struct {
	IntegerFormat *bool `mapstructure:"integer-format"`
	IntConversion *bool `mapstructure:"int-conversion"`

	ErrorFormat *bool `mapstructure:"error-format"`
	ErrError    *bool `mapstructure:"err-error"`
	ErrorF      *bool `mapstructure:"errorf"`

	StringFormat *bool `mapstructure:"string-format"`
	SprintF1     *bool `mapstructure:"sprintf1"`
	StrConcat    *bool `mapstructure:"strconcat"`

	BoolFormat *bool `mapstructure:"bool-format"`
	HexFormat  *bool `mapstructure:"hex-format"`
}

type PreallocSettings struct {
	Simple     *bool `mapstructure:"simple"`
	RangeLoops *bool `mapstructure:"range-loops"`
	ForLoops   *bool `mapstructure:"for-loops"`
}

type PredeclaredSettings struct {
	Ignore    *string `mapstructure:"ignore"`
	Qualified *bool   `mapstructure:"q"`
}

type PromlinterSettings struct {
	Strict          *bool    `mapstructure:"strict"`
	DisabledLinters []string `mapstructure:"disabled-linters"`
}

type ProtoGetterSettings struct {
	SkipGeneratedBy         []string `mapstructure:"skip-generated-by"`
	SkipFiles               []string `mapstructure:"skip-files"`
	SkipAnyGenerated        *bool    `mapstructure:"skip-any-generated"`
	ReplaceFirstArgInAppend *bool    `mapstructure:"replace-first-arg-in-append"`
}

type ReassignSettings struct {
	Patterns []string `mapstructure:"patterns"`
}

type RecvcheckSettings struct {
	DisableBuiltin *bool    `mapstructure:"disable-builtin"`
	Exclusions     []string `mapstructure:"exclusions"`
}

type ReviveSettings struct {
	Go                    *string  `mapstructure:"-"`
	MaxOpenFiles          *int     `mapstructure:"max-open-files"`
	IgnoreGeneratedHeader *bool    `mapstructure:"ignore-generated-header"`
	Confidence            *float64 `mapstructure:"confidence"`
	Severity              *string  `mapstructure:"severity"`
	EnableAllRules        *bool    `mapstructure:"enable-all-rules"`
	Rules                 []struct {
		Name      *string  `mapstructure:"name"`
		Arguments []any    `mapstructure:"arguments"`
		Severity  *string  `mapstructure:"severity"`
		Disabled  *bool    `mapstructure:"disabled"`
		Exclude   []string `mapstructure:"exclude"`
	} `mapstructure:"rules"`
	ErrorCode   *int `mapstructure:"error-code"`
	WarningCode *int `mapstructure:"warning-code"`
	Directives  []struct {
		Name     *string `mapstructure:"name"`
		Severity *string `mapstructure:"severity"`
	} `mapstructure:"directives"`
}

type RowsErrCheckSettings struct {
	Packages []string `mapstructure:"packages"`
}

type SlogLintSettings struct {
	NoMixedArgs    *bool    `mapstructure:"no-mixed-args"`
	KVOnly         *bool    `mapstructure:"kv-only"`
	AttrOnly       *bool    `mapstructure:"attr-only"`
	NoGlobal       *string  `mapstructure:"no-global"`
	Context        *string  `mapstructure:"context"`
	StaticMsg      *bool    `mapstructure:"static-msg"`
	NoRawKeys      *bool    `mapstructure:"no-raw-keys"`
	KeyNamingCase  *string  `mapstructure:"key-naming-case"`
	ForbiddenKeys  []string `mapstructure:"forbidden-keys"`
	ArgsOnSepLines *bool    `mapstructure:"args-on-sep-lines"`
}

type SpancheckSettings struct {
	Checks                   []string `mapstructure:"checks"`
	IgnoreCheckSignatures    []string `mapstructure:"ignore-check-signatures"`
	ExtraStartSpanSignatures []string `mapstructure:"extra-start-span-signatures"`
}

type StaticCheckSettings struct {
	Checks                  []string `mapstructure:"checks"`
	Initialisms             []string `mapstructure:"initialisms"`                // only for stylecheck
	DotImportWhitelist      []string `mapstructure:"dot-import-whitelist"`       // only for stylecheck
	HTTPStatusCodeWhitelist []string `mapstructure:"http-status-code-whitelist"` // only for stylecheck
}

type TagAlignSettings struct {
	Align  *bool    `mapstructure:"align"`
	Sort   *bool    `mapstructure:"sort"`
	Order  []string `mapstructure:"order"`
	Strict *bool    `mapstructure:"strict"`
}

type TagliatelleSettings struct {
	Case TagliatelleCase `mapstructure:"case"`
}

type TagliatelleCase struct {
	TagliatelleBase `mapstructure:",squash"`
	Overrides       []TagliatelleOverrides `mapstructure:"overrides"`
}

type TagliatelleOverrides struct {
	TagliatelleBase `mapstructure:",squash"`
	Package         *string `mapstructure:"pkg"`
	Ignore          *bool   `mapstructure:"ignore"`
}

type TagliatelleBase struct {
	Rules         map[string]string                  `mapstructure:"rules"`
	ExtendedRules map[string]TagliatelleExtendedRule `mapstructure:"extended-rules"`
	UseFieldName  *bool                              `mapstructure:"use-field-name"`
	IgnoredFields []string                           `mapstructure:"ignored-fields"`
}

type TagliatelleExtendedRule struct {
	Case                *string         `mapstructure:"case"`
	ExtraInitialisms    *bool           `mapstructure:"extra-initialisms"`
	InitialismOverrides map[string]bool `mapstructure:"initialism-overrides"`
}

type TestifylintSettings struct {
	EnableAll        *bool    `mapstructure:"enable-all"`
	DisableAll       *bool    `mapstructure:"disable-all"`
	EnabledCheckers  []string `mapstructure:"enable"`
	DisabledCheckers []string `mapstructure:"disable"`

	BoolCompare struct {
		IgnoreCustomTypes *bool `mapstructure:"ignore-custom-types"`
	} `mapstructure:"bool-compare"`

	ExpectedActual struct {
		ExpVarPattern *string `mapstructure:"pattern"`
	} `mapstructure:"expected-actual"`

	Formatter struct {
		CheckFormatString *bool `mapstructure:"check-format-string"`
		RequireFFuncs     *bool `mapstructure:"require-f-funcs"`
	} `mapstructure:"formatter"`

	GoRequire struct {
		IgnoreHTTPHandlers *bool `mapstructure:"ignore-http-handlers"`
	} `mapstructure:"go-require"`

	RequireError struct {
		FnPattern *string `mapstructure:"fn-pattern"`
	} `mapstructure:"require-error"`

	SuiteExtraAssertCall struct {
		Mode *string `mapstructure:"mode"`
	} `mapstructure:"suite-extra-assert-call"`
}

type TestpackageSettings struct {
	SkipRegexp    *string  `mapstructure:"skip-regexp"`
	AllowPackages []string `mapstructure:"allow-packages"`
}

type ThelperSettings struct {
	Test      ThelperOptions `mapstructure:"test"`
	Fuzz      ThelperOptions `mapstructure:"fuzz"`
	Benchmark ThelperOptions `mapstructure:"benchmark"`
	TB        ThelperOptions `mapstructure:"tb"`
}

type ThelperOptions struct {
	First *bool `mapstructure:"first"`
	Name  *bool `mapstructure:"name"`
	Begin *bool `mapstructure:"begin"`
}

type TenvSettings struct {
	All *bool `mapstructure:"all"`
}

type UseStdlibVarsSettings struct {
	HTTPMethod         *bool `mapstructure:"http-method"`
	HTTPStatusCode     *bool `mapstructure:"http-status-code"`
	TimeWeekday        *bool `mapstructure:"time-weekday"`
	TimeMonth          *bool `mapstructure:"time-month"`
	TimeLayout         *bool `mapstructure:"time-layout"`
	CryptoHash         *bool `mapstructure:"crypto-hash"`
	DefaultRPCPath     *bool `mapstructure:"default-rpc-path"`
	SQLIsolationLevel  *bool `mapstructure:"sql-isolation-level"`
	TLSSignatureScheme *bool `mapstructure:"tls-signature-scheme"`
	ConstantKind       *bool `mapstructure:"constant-kind"`

	// Deprecated
	OSDevNull *bool `mapstructure:"os-dev-null"`
	// Deprecated
	SyslogPriority *bool `mapstructure:"syslog-priority"`
}

type UseTestingSettings struct {
	ContextBackground *bool `mapstructure:"context-background"`
	ContextTodo       *bool `mapstructure:"context-todo"`
	OSChdir           *bool `mapstructure:"os-chdir"`
	OSMkdirTemp       *bool `mapstructure:"os-mkdir-temp"`
	OSSetenv          *bool `mapstructure:"os-setenv"`
	OSTempDir         *bool `mapstructure:"os-temp-dir"`
	OSCreateTemp      *bool `mapstructure:"os-create-temp"`
}

type UnconvertSettings struct {
	FastMath *bool `mapstructure:"fast-math"`
	Safe     *bool `mapstructure:"safe"`
}

type UnparamSettings struct {
	CheckExported *bool   `mapstructure:"check-exported"`
	Algo          *string `mapstructure:"algo"`
}

type UnusedSettings struct {
	FieldWritesAreUses     *bool `mapstructure:"field-writes-are-uses"`
	PostStatementsAreReads *bool `mapstructure:"post-statements-are-reads"`
	ExportedFieldsAreUsed  *bool `mapstructure:"exported-fields-are-used"`
	ParametersAreUsed      *bool `mapstructure:"parameters-are-used"`
	LocalVariablesAreUsed  *bool `mapstructure:"local-variables-are-used"`
	GeneratedIsUsed        *bool `mapstructure:"generated-is-used"`

	// Deprecated
	ExportedIsUsed *bool `mapstructure:"exported-is-used"`
}

type VarnamelenSettings struct {
	MaxDistance        *int     `mapstructure:"max-distance"`
	MinNameLength      *int     `mapstructure:"min-name-length"`
	CheckReceiver      *bool    `mapstructure:"check-receiver"`
	CheckReturn        *bool    `mapstructure:"check-return"`
	CheckTypeParam     *bool    `mapstructure:"check-type-param"`
	IgnoreNames        []string `mapstructure:"ignore-names"`
	IgnoreTypeAssertOk *bool    `mapstructure:"ignore-type-assert-ok"`
	IgnoreMapIndexOk   *bool    `mapstructure:"ignore-map-index-ok"`
	IgnoreChanRecvOk   *bool    `mapstructure:"ignore-chan-recv-ok"`
	IgnoreDecls        []string `mapstructure:"ignore-decls"`
}

type WhitespaceSettings struct {
	MultiIf   *bool `mapstructure:"multi-if"`
	MultiFunc *bool `mapstructure:"multi-func"`
}

type WrapcheckSettings struct {
	ExtraIgnoreSigs []string `mapstructure:"extra-ignore-sigs"`
	// TODO(ldez): v2 the options must be renamed to use hyphen.
	IgnoreSigs             []string `mapstructure:"ignoreSigs"`
	IgnoreSigRegexps       []string `mapstructure:"ignoreSigRegexps"`
	IgnorePackageGlobs     []string `mapstructure:"ignorePackageGlobs"`
	IgnoreInterfaceRegexps []string `mapstructure:"ignoreInterfaceRegexps"`
}

type WSLSettings struct {
	StrictAppend                     *bool    `mapstructure:"strict-append"`
	AllowAssignAndCallCuddle         *bool    `mapstructure:"allow-assign-and-call"`
	AllowAssignAndAnythingCuddle     *bool    `mapstructure:"allow-assign-and-anything"`
	AllowMultiLineAssignCuddle       *bool    `mapstructure:"allow-multiline-assign"`
	ForceCaseTrailingWhitespaceLimit *int     `mapstructure:"force-case-trailing-whitespace"`
	AllowTrailingComment             *bool    `mapstructure:"allow-trailing-comment"`
	AllowSeparatedLeadingComment     *bool    `mapstructure:"allow-separated-leading-comment"`
	AllowCuddleDeclaration           *bool    `mapstructure:"allow-cuddle-declarations"`
	AllowCuddleWithCalls             []string `mapstructure:"allow-cuddle-with-calls"`
	AllowCuddleWithRHS               []string `mapstructure:"allow-cuddle-with-rhs"`
	ForceCuddleErrCheckAndAssign     *bool    `mapstructure:"force-err-cuddling"`
	ErrorVariableNames               []string `mapstructure:"error-variable-names"`
	ForceExclusiveShortDeclarations  *bool    `mapstructure:"force-short-decl-cuddling"`
}

// CustomLinterSettings encapsulates the meta-data of a private linter.
type CustomLinterSettings struct {
	// Type plugin type.
	// It can be `goplugin` or `module`.
	Type *string `mapstructure:"type"`

	// Path to a plugin *.so file that implements the private linter.
	// Only for Go plugin system.
	Path *string `mapstructure:"path"`

	// Description describes the purpose of the private linter.
	Description *string `mapstructure:"description"`
	// OriginalURL The URL containing the source code for the private linter.
	OriginalURL *string `mapstructure:"original-url"`

	// Settings plugin settings only work with linterdb.PluginConstructor symbol.
	Settings any `mapstructure:"settings"`
}

type GciSettings struct {
	Sections         []string `mapstructure:"sections"`
	NoInlineComments *bool    `mapstructure:"no-inline-comments"`
	NoPrefixComments *bool    `mapstructure:"no-prefix-comments"`
	SkipGenerated    *bool    `mapstructure:"skip-generated"`
	CustomOrder      *bool    `mapstructure:"custom-order"`
	NoLexOrder       *bool    `mapstructure:"no-lex-order"`

	// Deprecated: use Sections instead.
	LocalPrefixes *string `mapstructure:"local-prefixes"`
}

type GoFmtSettings struct {
	Simplify     *bool              `mapstructure:"simplify"`
	RewriteRules []GoFmtRewriteRule `mapstructure:"rewrite-rules"`
}

type GoFmtRewriteRule struct {
	Pattern     *string `mapstructure:"pattern"`
	Replacement *string `mapstructure:"replacement"`
}

type GoFumptSettings struct {
	ModulePath *string `mapstructure:"module-path"`
	ExtraRules *bool   `mapstructure:"extra-rules"`

	// Deprecated: use the global `run.go` instead.
	LangVersion *string `mapstructure:"lang-version"`
}

type GoImportsSettings struct {
	LocalPrefixes *string `mapstructure:"local-prefixes"`
}
