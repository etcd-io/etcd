# ExpToStd

Detects functions from golang.org/x/exp/ that can be replaced by std functions.

[![Sponsor](https://img.shields.io/badge/Sponsor%20me-%E2%9D%A4%EF%B8%8F-pink)](https://github.com/sponsors/ldez)

Actual detections:

- `golang.org/x/exp/maps`:
  - `Keys`
  - `Values`
  - `Equal`
  - `EqualFunc`
  - `Clone`
  - `Copy`
  - `DeleteFunc`
  - `Clear`

- `golang.org/x/exp/slices`:
  - `Equal`
  - `EqualFunc`
  - `Compare`
  - `CompareFunc`
  - `Index`
  - `IndexFunc`
  - `Contains`
  - `ContainsFunc`
  - `Insert`
  - `Delete`
  - `DeleteFunc`
  - `Replace`
  - `Clone`
  - `Compact`
  - `CompactFunc`
  - `Grow`
  - `Clip`
  - `Reverse`
  - `Sort`
  - `SortFunc`
  - `SortStableFunc`
  - `IsSorted`
  - `IsSortedFunc`
  - `Min`
  - `MinFunc`
  - `Max`
  - `MaxFunc`
  - `BinarySearch`
  - `BinarySearchFunc`

- `golang.org/x/exp/constraints`:
  - `Ordered`

## Usages

### Inside golangci-lint

Recommended.

```yaml
linters:
  enable:
    - exptostd
```

### As a CLI

```bash
go install github.com/ldez/exptostd/cmd/exptostd@latest
```

```bash
./exptostd ./...
```

## Examples

```go
package foo

import (
	"fmt"

	"golang.org/x/exp/maps"
)

func foo(m map[string]string) {
	clone := maps.Clone(m)

	fmt.Println(clone)
}
```

It can be replaced by:

```go
package foo

import (
	"fmt"
	"maps"
)

func foo(m map[string]string) {
	clone := maps.Clone(m)

	fmt.Println(clone)
}

```

## References

- https://tip.golang.org/doc/go1.21#maps
- https://tip.golang.org/doc/go1.21#slices
- https://tip.golang.org/doc/go1.23#iterators
- https://tip.golang.org/doc/go1.21#cmp
