#!/usr/bin/env bash

set -euo pipefail

ARTIFACT_DIR="${ARTIFACT_DIR:-${HOME}/bench-artifacts}"
RUN_GROUP="${RUN_GROUP:-kilt-capacity-fresh-$(date -u +%Y%m%dT%H%M%SZ)}"
KEY_PREFIX="${KEY_PREFIX:-kilt-bench/${RUN_GROUP}/}"
LOG_PATH="${LOG_PATH:-${ARTIFACT_DIR}/${RUN_GROUP}-driver.log}"

mkdir -p "${ARTIFACT_DIR}"

log() {
	printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

run_ramp_group() {
	local scenarios="$1"
	log "Starting scenarios=${scenarios} RUN_GROUP=${RUN_GROUP} KEY_PREFIX=${KEY_PREFIX}"
	env RUN_GROUP="${RUN_GROUP}" KEY_PREFIX="${KEY_PREFIX}" SCENARIOS="${scenarios}" AUTO_RUN=true ./kilt-capacity-benchmark-ramp.sh
	log "Finished scenarios=${scenarios}"
}

reset_cluster() {
	log "Resetting etcd cluster before next scenario group"
	./kilt-recluster-etcd.sh --apply --skip-firewall
	log "Cluster reset complete"
}

{
	log "Fresh capacity benchmark batch starting"
	log "Artifacts: ${ARTIFACT_DIR}"
	log "Run group: ${RUN_GROUP}"
	log "Key prefix: ${KEY_PREFIX}"

	run_ramp_group "put,range"
	reset_cluster

	run_ramp_group "txn-put"
	reset_cluster

	run_ramp_group "watch-latency,mixed"
	reset_cluster

	log "Fresh capacity benchmark batch complete"
	log "Full summary markdown: ${ARTIFACT_DIR}/${RUN_GROUP}-full-summary.md"
	log "Full summary HTML: ${ARTIFACT_DIR}/${RUN_GROUP}-full-summary.html"
} 2>&1 | tee -a "${LOG_PATH}"
