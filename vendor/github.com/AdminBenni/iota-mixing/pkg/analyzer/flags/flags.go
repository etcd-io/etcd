package flags

import "flag"

const (
	TrueString  = "true"
	FalseString = "false"

	ReportIndividualFlagName  = "report-individual"
	reportIndividualFlagUsage = "whether or not to report individual consts rather than just the const block."
)

var (
	reportIndividualFlag *string //nolint:gochecknoglobals // only used in this file, not too plussed
)

func SetupFlags(flags *flag.FlagSet) {
	reportIndividualFlag = flags.String(ReportIndividualFlagName, FalseString, reportIndividualFlagUsage)
}

func ReportIndividualFlag() string {
	return *reportIndividualFlag
}
