// (c) Copyright gosec's authors
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

package analyzers

import "golang.org/x/tools/go/ssa"

type dependencyKey struct {
	value  ssa.Value
	target ssa.Value
}

type dependencyChecker struct {
	memo     map[dependencyKey]bool
	visiting map[dependencyKey]struct{}
}

func newDependencyChecker() *dependencyChecker {
	return &dependencyChecker{
		memo:     make(map[dependencyKey]bool),
		visiting: make(map[dependencyKey]struct{}),
	}
}

func (c *dependencyChecker) dependsOn(value ssa.Value, target ssa.Value) bool {
	return c.dependsOnDepth(value, target, 0)
}

func (c *dependencyChecker) dependsOnDepth(value ssa.Value, target ssa.Value, depth int) bool {
	if value == nil || target == nil || depth > MaxDepth {
		return false
	}
	if value == target {
		return true
	}

	key := dependencyKey{value: value, target: target}
	if result, ok := c.memo[key]; ok {
		return result
	}
	if _, ok := c.visiting[key]; ok {
		return false
	}

	c.visiting[key] = struct{}{}
	result := false

	switch v := value.(type) {
	case *ssa.ChangeType:
		result = c.dependsOnDepth(v.X, target, depth+1)
	case *ssa.MakeInterface:
		result = c.dependsOnDepth(v.X, target, depth+1)
	case *ssa.TypeAssert:
		result = c.dependsOnDepth(v.X, target, depth+1)
	case *ssa.UnOp:
		result = c.dependsOnDepth(v.X, target, depth+1)
	case *ssa.FieldAddr:
		result = c.dependsOnDepth(v.X, target, depth+1)
	case *ssa.Field:
		result = c.dependsOnDepth(v.X, target, depth+1)
	case *ssa.IndexAddr:
		result = c.dependsOnDepth(v.X, target, depth+1) || c.dependsOnDepth(v.Index, target, depth+1)
	case *ssa.Index:
		result = c.dependsOnDepth(v.X, target, depth+1) || c.dependsOnDepth(v.Index, target, depth+1)
	case *ssa.Slice:
		if c.dependsOnDepth(v.X, target, depth+1) {
			result = true
			break
		}
		if v.Low != nil && c.dependsOnDepth(v.Low, target, depth+1) {
			result = true
			break
		}
		if v.High != nil && c.dependsOnDepth(v.High, target, depth+1) {
			result = true
			break
		}
		result = v.Max != nil && c.dependsOnDepth(v.Max, target, depth+1)
	case *ssa.Extract:
		result = c.dependsOnDepth(v.Tuple, target, depth+1)
	case *ssa.Phi:
		for _, edge := range v.Edges {
			if c.dependsOnDepth(edge, target, depth+1) {
				result = true
				break
			}
		}
	case *ssa.Call:
		if v.Call.Value != nil && c.dependsOnDepth(v.Call.Value, target, depth+1) {
			result = true
			break
		}
		for _, arg := range v.Call.Args {
			if c.dependsOnDepth(arg, target, depth+1) {
				result = true
				break
			}
		}
	}

	delete(c.visiting, key)
	c.memo[key] = result

	return result
}
