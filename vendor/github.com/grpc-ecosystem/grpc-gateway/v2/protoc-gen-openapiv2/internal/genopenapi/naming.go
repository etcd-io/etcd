package genopenapi

import (
	"reflect"
	"regexp"
	"strings"
)

// LookupNamingStrategy looks up the given naming strategy and returns the naming
// strategy function for it. The naming strategy function takes in the list of all
// fully-qualified proto message names, and returns a mapping from fully-qualified
// name to OpenAPI name.
func LookupNamingStrategy(strategyName string) func([]string) map[string]string {
	switch strings.ToLower(strategyName) {
	case "fqn":
		return resolveNamesFQN
	case "legacy":
		return resolveNamesLegacy
	case "simple":
		return resolveNamesSimple
	case "package":
		return resolveNamesPackage
	}
	return nil
}

// resolveNamesFQN uses the fully-qualified proto message name as the
// OpenAPI name, stripping the leading dot.
func resolveNamesFQN(messages []string) map[string]string {
	uniqueNames := make(map[string]string, len(messages))
	for _, p := range messages {
		// strip leading dot from proto fqn
		uniqueNames[p] = p[1:]
	}
	return uniqueNames
}

// resolveNamesLegacy takes the names of all proto messages and generates unique references by
// applying the legacy heuristics for deriving unique names: starting from the bottom of the name hierarchy, it
// determines the minimum number of components necessary to yield a unique name, adds one
// to that number, and then concatenates those last components with no separator in between
// to form a unique name.
//
// E.g., if the fully qualified name is `.a.b.C.D`, and there are other messages with fully
// qualified names ending in `.D` but not in `.C.D`, it assigns the unique name `bCD`.
func resolveNamesLegacy(messages []string) map[string]string {
	return resolveNamesUniqueWithContext(messages, 1, "", false)
}

// resolveNamesSimple takes the names of all proto messages and generates unique references by using a simple
// heuristic: starting from the bottom of the name hierarchy, it determines the minimum
// number of components necessary to yield a unique name, and then concatenates those last
// components with a "." separator in between to form a unique name.
//
// E.g., if the fully qualified name is `.a.b.C.D`, and there are other messages with
// fully qualified names ending in `.D` but not in `.C.D`, it assigns the unique name `C.D`.
func resolveNamesSimple(messages []string) map[string]string {
	return resolveNamesUniqueWithContext(messages, 0, ".", false)
}

// resolveNamesPackage takes the names of all proto messages and generates unique references by
// starting with the package-scoped name (with nested message types qualified by their containing
// "parent" types), and then following the "simple" heuristic above to add package name components
// until each message has a unique name with a "." between each component.
//
// E.g., if the fully qualified name is `.a.b.C.D`, the name is `C.D` unless there is another
// package-scoped name ending in "C.D", in  which case it would be `b.C.D` (unless that also
// conflicted, in which case the name would be the fully-qualified `a.b.C`).
func resolveNamesPackage(messages []string) map[string]string {
	return resolveNamesUniqueWithContext(messages, 0, ".", true)
}

// For the "package" naming strategy, we rely on the convention that package names are lowercase
// but message names are capitalized.
var pkgEndRegexp = regexp.MustCompile(`\.[A-Z]`)

// Take the names of every proto message and generates a unique reference by:
// first, separating each message name into its components by splitting at dots. Then,
// take the shortest suffix slice from each components slice that is unique among all
// messages, and convert it into a component name by taking extraContext additional
// components into consideration and joining all components with componentSeparator.
func resolveNamesUniqueWithContext(messages []string, extraContext int, componentSeparator string, qualifyNestedMessages bool) map[string]string {
	packagesByDepth := make(map[int][][]string)
	uniqueNames := make(map[string]string)

	hierarchy := func(pkg string) []string {
		if !qualifyNestedMessages {
			return strings.Split(pkg, ".")
		}
		pkgEnd := pkgEndRegexp.FindStringIndex(pkg)
		if pkgEnd == nil {
			// Fall back to non-qualified behavior if search based on convention fails.
			return strings.Split(pkg, ".")
		}
		// Return each package component as an element, followed by the full message name
		// (potentially qualified, if nested) as a single element.
		qualifiedPkgName := pkg[:pkgEnd[0]]
		nestedTypeName := pkg[pkgEnd[0]+1:]
		return append(strings.Split(qualifiedPkgName, "."), nestedTypeName)
	}

	for _, p := range messages {
		h := hierarchy(p)
		for depth := range h {
			if _, ok := packagesByDepth[depth]; !ok {
				packagesByDepth[depth] = make([][]string, 0)
			}
			packagesByDepth[depth] = append(packagesByDepth[depth], h[len(h)-depth:])
		}
	}

	count := func(list [][]string, item []string) int {
		i := 0
		for _, element := range list {
			if reflect.DeepEqual(element, item) {
				i++
			}
		}
		return i
	}

	for _, p := range messages {
		h := hierarchy(p)
		depth := 0
		for ; depth < len(h); depth++ {
			// depth + extraContext > 0 ensures that we only break for values of depth when the
			// resulting slice of name components is non-empty. Otherwise, we would return the
			// empty string as the concise unique name is len(messages) == 1 (which is
			// technically correct).
			if depth+extraContext > 0 && count(packagesByDepth[depth], h[len(h)-depth:]) == 1 {
				break
			}
		}
		start := len(h) - depth - extraContext
		if start < 0 {
			start = 0
		}
		components := h[start:]
		// When using empty separator (legacy mode), apply camelCase by title-casing
		// intermediate lowercase package components. E.g., "google.rpc.Status" -> "googleRpcStatus"
		// We only title-case components that are entirely lowercase (package names),
		// not message names which are already PascalCase.
		// Skip the first non-empty component (keep it lowercase for camelCase).
		if componentSeparator == "" && len(components) > 1 {
			firstNonEmpty := -1
			for i := 0; i < len(components); i++ {
				if components[i] != "" {
					firstNonEmpty = i
					break
				}
			}
			for i := firstNonEmpty + 1; i < len(components); i++ {
				if len(components[i]) > 0 && components[i] == strings.ToLower(components[i]) {
					components[i] = strings.ToUpper(components[i][:1]) + components[i][1:]
				}
			}
		}
		uniqueNames[p] = strings.Join(components, componentSeparator)
	}
	return uniqueNames
}
