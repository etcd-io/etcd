#!/usr/bin/env bash

set -euo pipefail

PREFIX="${PREFIX:-kilt-bench/}"
BENCH_BIN="${BENCH_BIN:-${HOME}/benchmark-2pc-linux}"
WIPE_ALL="${WIPE_ALL:-false}"
CONFIRM_WIPE_ALL="${CONFIRM_WIPE_ALL:-false}"
RESTART_AFTER_DEFRAG="${RESTART_AFTER_DEFRAG:-false}"
COMMAND_TIMEOUT="${COMMAND_TIMEOUT:-120s}"
USE_REMOTE_SSH="${USE_REMOTE_SSH:-false}"
ETCDCTL_IMAGE="${ETCDCTL_IMAGE:-kilt-dev-docker-local.artifactory.oci.oraclecorp.com/kilt-etcd-server:0.3.4007}"
BATCH_DELETE_PREFIX="${BATCH_DELETE_PREFIX:-true}"
BATCH_DELETE_SIZE="${BATCH_DELETE_SIZE:-100}"
SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=8)

nodes=(
	10.201.162.151
	10.201.47.57
	10.201.98.50
	10.201.5.177
	10.201.117.10
)

first_node="${nodes[0]}"
ETCDCTL_ENDPOINTS="${ETCDCTL_ENDPOINTS:-http://10.201.162.151:2379,http://10.201.47.57:2379,http://10.201.98.50:2379,http://10.201.5.177:2379,http://10.201.117.10:2379}"

log() {
	printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

quote() {
	printf '%q' "$1"
}

remote() {
	local host="$1"
	shift
	ssh "${SSH_OPTS[@]}" "opc@${host}" "$@"
}

remote_etcdctl() {
	local host="$1"
	shift
	remote "${host}" "sudo podman exec etcd-server etcdctl --endpoints=http://127.0.0.1:2379 --command-timeout=${COMMAND_TIMEOUT} $*"
}

local_etcdctl() {
	if command -v etcdctl >/dev/null 2>&1; then
		ETCDCTL_API=3 etcdctl --endpoints="${ETCDCTL_ENDPOINTS}" --command-timeout="${COMMAND_TIMEOUT}" "$@"
		return
	fi
	sudo podman run --rm --network host --entrypoint etcdctl "${ETCDCTL_IMAGE}" \
		--endpoints="${ETCDCTL_ENDPOINTS}" \
		--command-timeout="${COMMAND_TIMEOUT}" \
		"$@"
}

local_etcdctl_endpoint() {
	local endpoint="$1"
	shift
	if command -v etcdctl >/dev/null 2>&1; then
		ETCDCTL_API=3 etcdctl --endpoints="${endpoint}" --command-timeout="${COMMAND_TIMEOUT}" "$@"
		return
	fi
	sudo podman run --rm --network host --entrypoint etcdctl "${ETCDCTL_IMAGE}" \
		--endpoints="${endpoint}" \
		--command-timeout="${COMMAND_TIMEOUT}" \
		"$@"
}

cluster_etcdctl() {
	if [[ "${USE_REMOTE_SSH}" == "true" ]]; then
		remote_etcdctl "${first_node}" "$*"
		return
	fi
	local_etcdctl "$@"
}

show_node_snapshot() {
	local host="$1"
	log "Snapshot for ${host}"
	if [[ "${USE_REMOTE_SSH}" != "true" ]]; then
		echo "--- endpoint ---"
		echo "http://${host}:2379"
		echo '--- health ---'
		tmp="$(mktemp)"
		code="$(curl --max-time 3 -sS -o "${tmp}" -w '%{http_code}' "http://${host}:2379/health" 2>&1)"
		rc=$?
		echo "curl_rc=${rc} http_code=${code} body=$(cat "${tmp}" 2>/dev/null)"
		rm -f "${tmp}"
		echo '--- metrics ---'
		curl --max-time 5 -sS "http://${host}:2379/metrics" 2>/dev/null |
			egrep '^etcd_mvcc_db_total_size_in_bytes |^etcd_mvcc_db_total_size_in_use_in_bytes |^etcd_server_quota_backend_bytes ' || true
		echo '--- endpoint status ---'
		local_etcdctl_endpoint "http://${host}:2379" endpoint status -w table || true
		echo '--- alarms ---'
		local_etcdctl_endpoint "http://${host}:2379" alarm list || true
		return
	fi
	remote "${host}" "
		set +e
		echo '--- hostname ---'
		hostname -f 2>/dev/null || hostname
		echo '--- health ---'
		tmp=\$(mktemp)
		code=\$(curl --max-time 3 -sS -o \"\$tmp\" -w '%{http_code}' http://127.0.0.1:2379/health 2>&1)
		rc=\$?
		echo \"curl_rc=\$rc http_code=\$code body=\$(cat \"\$tmp\" 2>/dev/null)\"
		rm -f \"\$tmp\"
		echo '--- metrics ---'
		curl --max-time 5 -sS http://127.0.0.1:2379/metrics 2>/dev/null |
			egrep '^etcd_mvcc_db_total_size_in_bytes |^etcd_mvcc_db_total_size_in_use_in_bytes |^etcd_server_quota_backend_bytes ' || true
		echo '--- endpoint status ---'
		sudo podman exec etcd-server etcdctl --endpoints=http://127.0.0.1:2379 endpoint status -w table || true
		echo '--- alarms ---'
		sudo podman exec etcd-server etcdctl --endpoints=http://127.0.0.1:2379 alarm list || true
	"
}

current_revision() {
	local json rev

	if [[ "${USE_REMOTE_SSH}" == "true" ]]; then
		json="$(remote_etcdctl "${first_node}" "get '' --from-key --limit=1 -w json" || true)"
	else
		json="$(cluster_etcdctl get "" --from-key --limit=1 -w json || true)"
	fi
	rev="$(printf '%s' "${json}" | tr -d '\n' | sed -n 's/.*\"revision\":\([0-9][0-9]*\).*/\1/p' | head -1)"
	if [[ -n "${rev}" ]]; then
		printf '%s\n' "${rev}"
		return 0
	fi

	if [[ "${USE_REMOTE_SSH}" == "true" ]]; then
		json="$(remote_etcdctl "${first_node}" "endpoint status -w json" || true)"
	else
		json="$(cluster_etcdctl endpoint status -w json || true)"
	fi
	rev="$(printf '%s' "${json}" | tr -d '\n' | sed -n 's/.*\"revision\":\([0-9][0-9]*\).*/\1/p' | head -1)"
	if [[ -n "${rev}" ]]; then
		printf '%s\n' "${rev}"
		return 0
	fi

	return 1
}

wait_for_node() {
	local host="$1"
	local i

	for i in {1..60}; do
		if remote "${host}" "systemctl is-active --quiet etcd && curl --max-time 3 -fsS http://127.0.0.1:2379/metrics >/dev/null"; then
			log "${host}: etcd is active and metrics are responsive"
			return 0
		fi
		log "${host}: waiting for etcd after restart (${i}/60)"
		sleep 2
	done

	log "${host}: etcd did not become ready after restart"
	show_node_snapshot "${host}" || true
	return 1
}

if [[ "${WIPE_ALL}" == "true" && "${CONFIRM_WIPE_ALL}" != "true" ]]; then
	cat >&2 <<EOF
Refusing full-cluster delete without confirmation.

This removes every etcd key, not only benchmark keys. If this is the dedicated
benchmark cluster and that is what you want, rerun with:

  WIPE_ALL=true CONFIRM_WIPE_ALL=true $(basename "$0")
EOF
	exit 2
fi

log "Cleaning etcd benchmark data"
if [[ "${USE_REMOTE_SSH}" == "true" ]]; then
	log "Mode: remote SSH to each node"
else
	log "Mode: local client endpoints via etcdctl container"
fi
if [[ "${WIPE_ALL}" == "true" ]]; then
	log "Mode: full keyspace wipe"
else
	log "Mode: prefix wipe for ${PREFIX}"
fi

log "Pre-clean snapshots"
for h in "${nodes[@]}"; do
	echo "===== PRE ${h} ====="
	show_node_snapshot "${h}" || true
done

log "Deleting keys"
if [[ "${WIPE_ALL}" == "true" ]]; then
	if [[ "${USE_REMOTE_SSH}" == "true" ]]; then
		remote_etcdctl "${first_node}" "del '' --from-key"
	else
		cluster_etcdctl del "" --from-key
	fi
else
	if [[ "${USE_REMOTE_SSH}" == "true" ]]; then
		quoted_prefix="$(quote "${PREFIX}")"
		remote_etcdctl "${first_node}" "del ${quoted_prefix} --prefix"
	elif [[ "${BATCH_DELETE_PREFIX}" == "true" && -x "${BENCH_BIN}" ]]; then
		"${BENCH_BIN}" --endpoints="${ETCDCTL_ENDPOINTS}" clean-prefix --batch-size="${BATCH_DELETE_SIZE}" "${PREFIX}"
	else
		cluster_etcdctl del "${PREFIX}" --prefix
	fi
fi

rev="$(current_revision)"
log "Compacting revision ${rev}"
if [[ "${USE_REMOTE_SSH}" == "true" ]]; then
	remote_etcdctl "${first_node}" "compact ${rev}" || true
else
	cluster_etcdctl compact "${rev}" || true
fi

log "Defragmenting every member one at a time"
for h in "${nodes[@]}"; do
	echo "===== DEFRAG ${h} ====="
	if [[ "${USE_REMOTE_SSH}" == "true" ]]; then
		remote_etcdctl "${h}" "defrag"
	else
		local_etcdctl_endpoint "http://${h}:2379" defrag
	fi
done

log "Disarming NOSPACE alarms"
if [[ "${USE_REMOTE_SSH}" == "true" ]]; then
	remote_etcdctl "${first_node}" "alarm disarm" || true
else
	cluster_etcdctl alarm disarm || true
fi

if [[ "${RESTART_AFTER_DEFRAG}" == "true" ]]; then
	if [[ "${USE_REMOTE_SSH}" != "true" ]]; then
		log "RESTART_AFTER_DEFRAG requires USE_REMOTE_SSH=true"
		exit 2
	fi
	log "Rolling restart requested to release process RSS/page-cache pressure"
	for h in "${nodes[@]}"; do
		echo "===== RESTART ${h} ====="
		remote "${h}" "sudo systemctl restart etcd"
		wait_for_node "${h}"
	done
fi

log "Post-clean snapshots"
for h in "${nodes[@]}"; do
	echo "===== POST ${h} ====="
	show_node_snapshot "${h}" || true
done

log "Done"
