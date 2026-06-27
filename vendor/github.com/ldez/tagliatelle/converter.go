package tagliatelle

import (
	"fmt"
	"maps"
	"strings"

	"github.com/ettle/strcase"
)

// https://github.com/dominikh/go-tools/blob/v0.5.1/config/config.go#L167-L175
//
//nolint:gochecknoglobals // For now I'll accept this, but I think will refactor to use a structure.
var staticcheckInitialisms = map[string]bool{
	"AMQP": true,
	"DB":   true,
	"GID":  true,
	"LHS":  false,
	"RHS":  false,
	"RTP":  true,
	"SIP":  true,
	"TS":   true,
}

// Converter is the signature of a case converter.
type Converter func(s string) string

// ConverterCallback allows to abstract `getSimpleConverter` and `ruleToConverter`.
type ConverterCallback func() (Converter, error)

func getSimpleConverter(c string) (Converter, error) {
	switch c {
	case "camel":
		return strcase.ToCamel, nil
	case "pascal":
		return strcase.ToPascal, nil
	case "kebab":
		return strcase.ToKebab, nil
	case "snake":
		return strcase.ToSnake, nil
	case "goCamel":
		return strcase.ToGoCamel, nil
	case "goPascal":
		return strcase.ToGoPascal, nil
	case "goKebab":
		return strcase.ToGoKebab, nil
	case "goSnake":
		return strcase.ToGoSnake, nil
	case "upperSnake":
		return strcase.ToSNAKE, nil
	case "header":
		return toHeader, nil
	case "upper":
		return strings.ToUpper, nil
	case "lower":
		return strings.ToLower, nil
	default:
		return nil, fmt.Errorf("unsupported case: %s", c)
	}
}

func toHeader(s string) string {
	return strcase.ToCase(s, strcase.TitleCase, '-')
}

func ruleToConverter(rule ExtendedRule) (Converter, error) {
	initialismOverrides := maps.Clone(rule.InitialismOverrides)

	if rule.ExtraInitialisms {
		for k, v := range staticcheckInitialisms {
			if _, found := initialismOverrides[k]; found {
				continue
			}

			initialismOverrides[k] = v
		}
	}

	caser := strcase.NewCaser(strings.HasPrefix(rule.Case, "go"), initialismOverrides, nil)

	switch strings.ToLower(strings.TrimPrefix(rule.Case, "go")) {
	case "camel":
		return caser.ToCamel, nil

	case "pascal":
		return caser.ToPascal, nil

	case "kebab":
		return caser.ToKebab, nil

	case "snake":
		return caser.ToSnake, nil

	case "uppersnake":
		return caser.ToSNAKE, nil

	case "header":
		return toHeaderCase(caser), nil

	case "upper":
		return func(s string) string {
			return caser.ToCase(s, strcase.UpperCase, 0)
		}, nil

	case "lower":
		return func(s string) string {
			return caser.ToCase(s, strcase.LowerCase, 0)
		}, nil

	default:
		return nil, fmt.Errorf("unsupported case: %s", rule.Case)
	}
}

func toHeaderCase(caser *strcase.Caser) Converter {
	return func(s string) string {
		return caser.ToCase(s, strcase.TitleCase, '-')
	}
}
