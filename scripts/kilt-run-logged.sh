#!/usr/bin/env bash

set -o pipefail

LOG_PATH="${LOG_PATH:-${HOME}/bench-artifacts/kilt-capacity-20260513-commands.log}"

if [[ "$#" -eq 0 ]]; then
	echo "usage: $(basename "$0") <command>"
	exit 2
fi

mkdir -p "$(dirname "${LOG_PATH}")"

cmd="$*"
started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

{
	echo
	echo "===== COMMAND START ${started_at} ====="
	echo "${cmd}"
	echo "===== OUTPUT ====="
} | tee -a "${LOG_PATH}"

set +e
bash -lc "${cmd}" 2>&1 | sed -u -E $'s/\r/\n/g; s/\x1B\\[[0-9;?]*[ -/]*[@-~]//g' | tee -a "${LOG_PATH}"
status="${PIPESTATUS[0]}"
set -e

ended_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
{
	echo "===== COMMAND END ${ended_at} status=${status} ====="
	echo
} | tee -a "${LOG_PATH}"

exit "${status}"
