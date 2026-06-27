package pgservice

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/lib/pq/internal/pqutil"
)

func FindService(path string, service string) (map[string]string, error) {
	fp, err := os.Open(path)
	if err != nil {
		if pqutil.ErrNotExists(err) {
			// libpq just returns "definition of service not found" if the
			// default file doesn't exist, but IMO that's confusing.
			return nil, fmt.Errorf("service file %q not found", path)
		}
		return nil, err
	}
	defer fp.Close()

	var (
		scan = bufio.NewScanner(fp)
		i    int
	)
	for scan.Scan() {
		i++
		line := strings.TrimSpace(scan.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		// [service] header that we want.
		if line[0] == '[' && line[len(line)-1] == ']' && strings.TrimSpace(line[1:len(line)-1]) == service {
			opts := make(map[string]string)
			for scan.Scan() {
				i++
				line := strings.TrimSpace(scan.Text())
				if line == "" || line[0] == '#' {
					continue
				}
				// Next header: our work here is done.
				if line[0] == '[' && line[len(line)-1] == ']' {
					return opts, nil
				}

				k, v, ok := strings.Cut(line, "=")
				if !ok {
					return nil, fmt.Errorf("line %d: missing '=' in %q", i, line)
				}
				k, v = strings.TrimSpace(k), strings.TrimSpace(v)
				if k == "" {
					return nil, fmt.Errorf("line %d: no value before '=' in %q", i, line)
				}
				opts[k] = v
			}
			if scan.Err() != nil {
				return nil, scan.Err()
			}
			return opts, nil
		}
	}
	if scan.Err() != nil {
		return nil, scan.Err()
	}

	return nil, fmt.Errorf("definition of service %q not found", service)
}
