// (c) Copyright 2016 Hewlett Packard Enterprise Development LP
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strconv"

	zxcvbn "github.com/ccojocar/zxcvbn-go"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type secretPattern struct {
	name   string
	regexp *regexp.Regexp
}

// entropyCacheKey is the cache key for entropy analysis results.
type entropyCacheKey string

// secretPatternCacheKey is the cache key for secret pattern scan results.
type secretPatternCacheKey string

var secretsPatterns = [...]secretPattern{
	{
		name:   "RSA private key",
		regexp: regexp.MustCompile(`-----BEGIN RSA PRIVATE KEY-----`),
	},
	{
		name:   "SSH (DSA) private key",
		regexp: regexp.MustCompile(`-----BEGIN DSA PRIVATE KEY-----`),
	},
	{
		name:   "SSH (EC) private key",
		regexp: regexp.MustCompile(`-----BEGIN EC PRIVATE KEY-----`),
	},
	{
		name:   "PGP private key block",
		regexp: regexp.MustCompile(`-----BEGIN PGP PRIVATE KEY BLOCK-----`),
	},
	{
		name:   "Slack Token",
		regexp: regexp.MustCompile(`xox[pborsa]-[0-9]{12}-[0-9]{12}-[0-9]{12}-[a-z0-9]{32}`),
	},
	{
		name:   "AWS API Key",
		regexp: regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	},
	{
		name:   "Amazon MWS Auth Token",
		regexp: regexp.MustCompile(`amzn\.mws\.[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
	},
	{
		name:   "AWS AppSync GraphQL Key",
		regexp: regexp.MustCompile(`da2-[a-z0-9]{26}`),
	},
	{
		name:   "GitHub personal access token",
		regexp: regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
	},
	{
		name:   "GitHub fine-grained access token",
		regexp: regexp.MustCompile(`github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}`),
	},
	{
		name:   "GitHub action temporary token",
		regexp: regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`),
	},
	{
		name:   "Google API Key", // Also Google Cloud Platform, Gmail, Drive, YouTube, etc.
		regexp: regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
	},

	{
		name:   "Google Cloud Platform OAuth", // Also Gmail, Drive, YouTube, etc.
		regexp: regexp.MustCompile(`[0-9]+-[0-9A-Za-z_]{32}\.apps\.googleusercontent\.com`),
	},

	{
		name:   "Google (GCP) Service-account",
		regexp: regexp.MustCompile(`"type": "service_account"`),
	},

	{
		name:   "Google OAuth Access Token",
		regexp: regexp.MustCompile(`ya29\.[0-9A-Za-z\-_]+`),
	},

	{
		name:   "Generic API Key",
		regexp: regexp.MustCompile(`[aA][pP][iI]_?[kK][eE][yY].*[''|"][0-9a-zA-Z]{32,45}[''|"]`),
	},
	{
		name:   "Generic Secret",
		regexp: regexp.MustCompile(`[sS][eE][cC][rR][eE][tT].*[''|"][0-9a-zA-Z]{32,45}[''|"]`),
	},
	{
		name:   "Heroku API Key",
		regexp: regexp.MustCompile(`[hH][eE][rR][oO][kK][uU].*[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}`),
	},
	{
		name:   "MailChimp API Key",
		regexp: regexp.MustCompile(`[0-9a-f]{32}-us[0-9]{1,2}`),
	},
	{
		name:   "Mailgun API Key",
		regexp: regexp.MustCompile(`key-[0-9a-zA-Z]{32}`),
	},
	{
		name:   "Password in URL",
		regexp: regexp.MustCompile(`[a-zA-Z]{3,10}://[a-zA-Z0-9\.\-\_\+]{1,64}:[a-zA-Z0-9\.\-\_\!\$\%\&\*\+\=\^\(\)]{1,128}@[a-zA-Z0-9\.\-\_]+(:[0-9]+)?(/[^"'\s]*)?(["'\s]|$)`),
	},
	{
		name:   "Slack Webhook",
		regexp: regexp.MustCompile(`https://hooks\.slack\.com/services/T[a-zA-Z0-9_]{8}/B[a-zA-Z0-9_]{8}/[a-zA-Z0-9_]{24}`),
	},
	{
		name:   "Stripe API Key",
		regexp: regexp.MustCompile(`sk_live_[0-9a-zA-Z]{24}`),
	},
	{
		name:   "Stripe Restricted API Key",
		regexp: regexp.MustCompile(`rk_live_[0-9a-zA-Z]{24}`),
	},
	{
		name:   "Square Access Token",
		regexp: regexp.MustCompile(`sq0atp-[0-9A-Za-z\-_]{22}`),
	},
	{
		name:   "Square OAuth Secret",
		regexp: regexp.MustCompile(`sq0csp-[0-9A-Za-z\-_]{43}`),
	},
	{
		name:   "Telegram Bot API Key",
		regexp: regexp.MustCompile(`[0-9]+:AA[0-9A-Za-z\-_]{33}`),
	},
	{
		name:   "Twilio API Key",
		regexp: regexp.MustCompile(`SK[0-9a-fA-F]{32}`),
	},
	{
		name:   "Twitter Access Token",
		regexp: regexp.MustCompile(`[tT][wW][iI][tT][tT][eE][rR].*[1-9][0-9]+-[0-9a-zA-Z]{40}`),
	},
	{
		name:   "Twitter OAuth",
		regexp: regexp.MustCompile(`[tT][wW][iI][tT][tT][eE][rR].*[''|"][0-9a-zA-Z]{35,44}[''|"]`),
	},
}

type credentials struct {
	issue.MetaData
	pattern          *regexp.Regexp
	entropyThreshold float64
	perCharThreshold float64
	truncate         int
	ignoreEntropy    bool
	minEntropyLength int
}

func truncate(s string, n int) string {
	if n > len(s) {
		return s
	}
	return s[:n]
}

func (r *credentials) isHighEntropyString(str string) bool {
	if len(str) < r.minEntropyLength {
		return false
	}
	s := truncate(str, r.truncate)
	key := entropyCacheKey(s)
	if val, ok := gosec.GlobalCache.Get(key); ok {
		return val.(bool)
	}

	info := zxcvbn.PasswordStrength(s, []string{})
	entropyPerChar := info.Entropy / float64(len(s))
	res := (info.Entropy >= r.entropyThreshold ||
		(info.Entropy >= (r.entropyThreshold/2) &&
			entropyPerChar >= r.perCharThreshold))
	gosec.GlobalCache.Add(key, res)
	return res
}

type secretResult struct {
	ok          bool
	patternName string
}

func (r *credentials) isSecretPattern(str string) (bool, string) {
	if len(str) < r.minEntropyLength {
		return false, ""
	}
	key := secretPatternCacheKey(str)
	if res, ok := gosec.GlobalCache.Get(key); ok {
		secretRes := res.(secretResult)
		return secretRes.ok, secretRes.patternName
	}
	for _, pattern := range secretsPatterns {
		if gosec.RegexMatchWithCache(pattern.regexp, str) {
			gosec.GlobalCache.Add(key, secretResult{true, pattern.name})
			return true, pattern.name
		}
	}
	gosec.GlobalCache.Add(key, secretResult{false, ""})
	return false, ""
}

func (r *credentials) Match(n ast.Node, ctx *gosec.Context) (*issue.Issue, error) {
	switch node := n.(type) {
	case *ast.AssignStmt:
		return r.matchAssign(node, ctx)
	case *ast.ValueSpec:
		return r.matchValueSpec(node, ctx)
	case *ast.BinaryExpr:
		return r.matchEqualityCheck(node, ctx)
	case *ast.CompositeLit:
		return r.matchCompositeLit(node, ctx)
	}
	return nil, nil
}

func (r *credentials) matchAssign(assign *ast.AssignStmt, ctx *gosec.Context) (*issue.Issue, error) {
	for _, i := range assign.Lhs {
		if ident, ok := i.(*ast.Ident); ok {
			// First check LHS to find anything being assigned to variables whose name appears to be a cred
			if gosec.RegexMatchWithCache(r.pattern, ident.Name) {
				for _, e := range assign.Rhs {
					if val, err := gosec.GetString(e); err == nil {
						if r.ignoreEntropy || (!r.ignoreEntropy && r.isHighEntropyString(val)) {
							return ctx.NewIssue(assign, r.ID(), r.What, r.Severity, r.Confidence), nil
						}
					}
				}
			}

			// Now that no names were matched, match the RHS to see if the actual values being assigned are creds
			for _, e := range assign.Rhs {
				val, err := gosec.GetString(e)
				if err != nil {
					continue
				}

				if r.ignoreEntropy || r.isHighEntropyString(val) {
					if ok, patternName := r.isSecretPattern(val); ok {
						return ctx.NewIssue(assign, r.ID(), fmt.Sprintf("%s: %s", r.What, patternName), r.Severity, r.Confidence), nil
					}
				}
			}
		}
	}
	return nil, nil
}

func (r *credentials) matchValueSpec(valueSpec *ast.ValueSpec, ctx *gosec.Context) (*issue.Issue, error) {
	// Running match against the variable name(s) first. Will catch any creds whose var name matches the pattern,
	// then will go back over to check the values themselves.
	for index, ident := range valueSpec.Names {
		if gosec.RegexMatchWithCache(r.pattern, ident.Name) && valueSpec.Values != nil {
			// const foo, bar = "same value"
			if len(valueSpec.Values) <= index {
				index = len(valueSpec.Values) - 1
			}
			if val, err := gosec.GetString(valueSpec.Values[index]); err == nil {
				if r.ignoreEntropy || (!r.ignoreEntropy && r.isHighEntropyString(val)) {
					return ctx.NewIssue(valueSpec, r.ID(), r.What, r.Severity, r.Confidence), nil
				}
			}
		}
	}

	// Now that no variable names have been matched, match the actual values to find any creds
	for _, ident := range valueSpec.Values {
		if val, err := gosec.GetString(ident); err == nil {
			if r.ignoreEntropy || r.isHighEntropyString(val) {
				if ok, patternName := r.isSecretPattern(val); ok {
					return ctx.NewIssue(valueSpec, r.ID(), fmt.Sprintf("%s: %s", r.What, patternName), r.Severity, r.Confidence), nil
				}
			}
		}
	}

	return nil, nil
}

func (r *credentials) matchEqualityCheck(binaryExpr *ast.BinaryExpr, ctx *gosec.Context) (*issue.Issue, error) {
	if binaryExpr.Op == token.EQL || binaryExpr.Op == token.NEQ {
		ident, ok := binaryExpr.X.(*ast.Ident)
		if !ok {
			ident, _ = binaryExpr.Y.(*ast.Ident)
		}

		if ident != nil && gosec.RegexMatchWithCache(r.pattern, ident.Name) {
			valueNode := binaryExpr.Y
			if !ok {
				valueNode = binaryExpr.X
			}
			if val, err := gosec.GetString(valueNode); err == nil {
				if r.ignoreEntropy || (!r.ignoreEntropy && r.isHighEntropyString(val)) {
					return ctx.NewIssue(binaryExpr, r.ID(), r.What, r.Severity, r.Confidence), nil
				}
			}
		}

		// Now that the variable names have been checked, and no matches were found, make sure that
		// either the left or right operands is a string literal so we can match the value.
		identStrConst, ok := binaryExpr.X.(*ast.BasicLit)
		if !ok {
			identStrConst, ok = binaryExpr.Y.(*ast.BasicLit)
		}

		if ok && identStrConst.Kind == token.STRING {
			s, _ := gosec.GetString(identStrConst)
			if r.ignoreEntropy || r.isHighEntropyString(s) {
				if ok, patternName := r.isSecretPattern(s); ok {
					return ctx.NewIssue(binaryExpr, r.ID(), fmt.Sprintf("%s: %s", r.What, patternName), r.Severity, r.Confidence), nil
				}
			}
		}
	}
	return nil, nil
}

func (r *credentials) matchCompositeLit(lit *ast.CompositeLit, ctx *gosec.Context) (*issue.Issue, error) {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			// Check if the key matches the credential pattern (struct field name or map string literal key)
			matchedKey := false
			if ident, ok := kv.Key.(*ast.Ident); ok {
				if gosec.RegexMatchWithCache(r.pattern, ident.Name) {
					matchedKey = true
				}
			}
			if keyStr, err := gosec.GetString(kv.Key); err == nil {
				if gosec.RegexMatchWithCache(r.pattern, keyStr) {
					matchedKey = true
				}
			}

			// If key matches, check value for high entropy (generic credential warning)
			if matchedKey {
				if val, err := gosec.GetString(kv.Value); err == nil {
					if r.ignoreEntropy || r.isHighEntropyString(val) {
						return ctx.NewIssue(lit, r.ID(), r.What, r.Severity, r.Confidence), nil
					}
				}
			}

			// Separately check value for specific secret patterns (regardless of key)
			if val, err := gosec.GetString(kv.Value); err == nil {
				if r.ignoreEntropy || r.isHighEntropyString(val) {
					if ok, patternName := r.isSecretPattern(val); ok {
						return ctx.NewIssue(lit, r.ID(), fmt.Sprintf("%s: %s", r.What, patternName), r.Severity, r.Confidence), nil
					}
				}
			}
		}
	}
	return nil, nil
}

// NewHardcodedCredentials attempts to find high entropy string constants being
// assigned to variables that appear to be related to credentials.
func NewHardcodedCredentials(id string, conf gosec.Config) (gosec.Rule, []ast.Node) {
	pattern := `(?i)passwd|pass|password|pwd|secret|token|pw|apiKey|bearer|cred`
	entropyThreshold := 80.0
	perCharThreshold := 3.0
	ignoreEntropy := false
	truncateString := 16
	minEntropyLength := 8
	if val, ok := conf[id]; ok {
		conf := val.(map[string]interface{})
		if configPattern, ok := conf["pattern"]; ok {
			if cfgPattern, ok := configPattern.(string); ok {
				pattern = cfgPattern
			}
		}

		if configIgnoreEntropy, ok := conf["ignore_entropy"]; ok {
			if cfgIgnoreEntropy, ok := configIgnoreEntropy.(bool); ok {
				ignoreEntropy = cfgIgnoreEntropy
			}
		}
		if configEntropyThreshold, ok := conf["entropy_threshold"]; ok {
			if cfgEntropyThreshold, ok := configEntropyThreshold.(string); ok {
				if parsedNum, err := strconv.ParseFloat(cfgEntropyThreshold, 64); err == nil {
					entropyThreshold = parsedNum
				}
			}
		}
		if configCharThreshold, ok := conf["per_char_threshold"]; ok {
			if cfgCharThreshold, ok := configCharThreshold.(string); ok {
				if parsedNum, err := strconv.ParseFloat(cfgCharThreshold, 64); err == nil {
					perCharThreshold = parsedNum
				}
			}
		}
		if configTruncate, ok := conf["truncate"]; ok {
			if cfgTruncate, ok := configTruncate.(string); ok {
				if parsedInt, err := strconv.Atoi(cfgTruncate); err == nil {
					truncateString = parsedInt
				}
			}
		}
		if configMinEntropyLength, ok := conf["min_entropy_length"]; ok {
			if cfgMinEntropyLength, ok := configMinEntropyLength.(string); ok {
				if parsedInt, err := strconv.Atoi(cfgMinEntropyLength); err == nil {
					minEntropyLength = parsedInt
				}
			}
		}
	}

	return &credentials{
		pattern:          regexp.MustCompile(pattern),
		entropyThreshold: entropyThreshold,
		perCharThreshold: perCharThreshold,
		ignoreEntropy:    ignoreEntropy,
		truncate:         truncateString,
		minEntropyLength: minEntropyLength,
		MetaData:         issue.NewMetaData(id, "Potential hardcoded credentials", issue.High, issue.Low),
	}, []ast.Node{(*ast.AssignStmt)(nil), (*ast.ValueSpec)(nil), (*ast.BinaryExpr)(nil), (*ast.CompositeLit)(nil)}
}
