package pb

var (
	// Full - preset with all default available elements
	// Example: 'Prefix 20/100 [-->______] 20% 1 p/s ETA 1m Suffix'
	Full ProgressBarTemplate = `{{with string . "prefix"}}{{.}} {{end}}{{counters . }} {{bar . }} {{percent . }} {{speed . }} {{rtime . "ETA %s"}}{{with string . "suffix"}} {{.}}{{end}}`

	// Default - preset like Full but without elapsed time
	// Example: 'Prefix 20/100 [-->______] 20% 1 p/s Suffix'
	Default ProgressBarTemplate = `{{with string . "prefix"}}{{.}} {{end}}{{counters . }} {{bar . }} {{percent . }} {{speed . }}{{with string . "suffix"}} {{.}}{{end}}`

	// Simple - preset without speed and any timers. Only counters, bar and percents
	// Example: 'Prefix 20/100 [-->______] 20% Suffix'
	Simple ProgressBarTemplate = `{{with string . "prefix"}}{{.}} {{end}}{{counters . }} {{bar . }} {{percent . }}{{with string . "suffix"}} {{.}}{{end}}`
)
