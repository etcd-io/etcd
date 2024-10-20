#!/usr/bin/env bash

set -euo pipefail

cd ./tools/mod || exit 2
go list --tags tools -f '{{ join .Imports "\n" }}' | xargs go install
