package sarif

const Version = "2.1.0"
const Schema = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"

type Log struct {
	Version string `json:"version"`
	Schema  string `json:"$schema"`
	Runs    []Run  `json:"runs"`
}

type Run struct {
	Tool        Tool         `json:"tool"`
	Results     []Result     `json:"results,omitempty"`
	Invocations []Invocation `json:"invocations,omitempty"`
	Artifacts   []Artifact   `json:"artifacts,omitempty"`
}

type Artifact struct {
	Location       ArtifactLocation `json:"location"`
	Length         int              `json:"length"`
	SourceLanguage string           `json:"sourceLanguage"`
	Roles          []string         `json:"roles"`
	Encoding       string           `json:"encoding"`
}

const (
	AnalysisTarget = "analysisTarget"
	UTF8           = "UTF-8"
	Fail           = "fail"
	Warning        = "warning"
	Error          = "error"
	Note           = "note"
	None           = "none"
)

type Hash struct {
	Sha256 string `json:"sha-256"`
}

type Tool struct {
	Driver ToolComponent `json:"driver"`
}

type Invocation struct {
	CommandLine         string           `json:"commandLine,omitempty"`
	Arguments           []string         `json:"arguments,omitempty"`
	WorkingDirectory    ArtifactLocation `json:"workingDirectory,omitzero"`
	ExecutionSuccessful bool             `json:"executionSuccessful"`
}

type ToolComponent struct {
	Name            string                `json:"name,omitempty"`
	Version         string                `json:"version,omitempty"`
	SemanticVersion string                `json:"semanticVersion,omitempty"`
	InformationURI  string                `json:"informationUri,omitempty"`
	Rules           []ReportingDescriptor `json:"rules,omitempty"`
}

type ReportingDescriptor struct {
	ID               string  `json:"id"`
	ShortDescription Message `json:"shortDescription"`
	// FullDescription  Message `json:"fullDescription"`
	Help                 Message                `json:"help"`
	HelpURI              string                 `json:"helpUri,omitempty"`
	DefaultConfiguration ReportingConfiguration `json:"defaultConfiguration"`
}

type ReportingConfiguration struct {
	Enabled    bool           `json:"enabled"`
	Level      string         `json:"level,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type Result struct {
	RuleID string `json:"ruleId"`
	// RuleIndex        int        `json:"ruleIndex"`
	Kind             string        `json:"kind"`
	Level            string        `json:"level,omitempty"`
	Message          Message       `json:"message"`
	Locations        []Location    `json:"locations,omitempty"`
	RelatedLocations []Location    `json:"relatedLocations,omitempty"`
	Fixes            []Fix         `json:"fixes,omitempty"`
	Suppressions     []Suppression `json:"suppressions"`
}

type Suppression struct {
	Kind          string `json:"kind"`
	Justification string `json:"justification"`
}

type Fix struct {
	Description     Message          `json:"description"`
	ArtifactChanges []ArtifactChange `json:"artifactChanges"`
}

type ArtifactChange struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Replacements     []Replacement    `json:"replacements"`
}

type Replacement struct {
	DeletedRegion   Region          `json:"deletedRegion"`
	InsertedContent ArtifactContent `json:"insertedContent"`
}

type ArtifactContent struct {
	Text string `json:"text"`
}

type Message struct {
	Text     string `json:"text,omitempty"`
	Markdown string `json:"markdown,omitempty"`
}

type Location struct {
	ID               int              `json:"id,omitempty"`
	Message          *Message         `json:"message,omitempty"`
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           Region           `json:"region"`
}

type ArtifactLocation struct {
	URI       string `json:"uri,omitempty"`
	Index     int    `json:"index,omitempty"`
	URIBaseID string `json:"uriBaseId,omitempty"`
}

type Region struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}
