#!/usr/bin/env bash

set -euo pipefail

QUOTA_BYTES="${QUOTA_BYTES:-8589934592}"
SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=8)

nodes=(
	10.201.162.151
	10.201.47.57
	10.201.98.50
	10.201.5.177
	10.201.117.10
)

log() {
	printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

remote() {
	local host="$1"
	shift
	ssh "${SSH_OPTS[@]}" "opc@${host}" "$@"
}

remote_etcdctl() {
	local host="$1"
	shift
	remote "${host}" "sudo podman exec etcd-server sh -lc 'export ETCDCTL_API=3; etcdctl --endpoints=http://127.0.0.1:2379 $*'"
}

show_node_snapshot() {
	local host="$1"
	log "Snapshot for ${host}"
	remote "${host}" "
		set +e
		echo '--- hostname ---'
		hostname -f 2>/dev/null || hostname
		echo '--- systemd ---'
		systemctl is-enabled etcd 2>/dev/null || true
		systemctl is-active etcd 2>/dev/null || true
		systemctl --no-pager --full status etcd 2>/dev/null | sed -n '1,12p' || true
		echo '--- quota env in source ---'
		grep '^Environment=ETCD_QUOTA_BACKEND_BYTES=' /etc/containers/systemd/etcd.container || echo 'Environment=ETCD_QUOTA_BACKEND_BYTES=<unset>'
		echo '--- quota env in generated unit ---'
		systemctl cat etcd 2>/dev/null | grep 'ETCD_QUOTA_BACKEND_BYTES' || echo 'ETCD_QUOTA_BACKEND_BYTES=<unset>'
		echo '--- container ---'
		sudo podman ps --filter name=etcd-server --format 'name={{.Names}} status={{.Status}} image={{.Image}}' || true
		echo '--- quota env in running container ---'
		sudo podman inspect etcd-server --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null | grep '^ETCD_QUOTA_BACKEND_BYTES=' || echo 'ETCD_QUOTA_BACKEND_BYTES=<unset>'
		echo '--- health ---'
		tmp=\$(mktemp)
		code=\$(curl --max-time 3 -sS -o \"\$tmp\" -w '%{http_code}' http://127.0.0.1:2379/health 2>&1)
		rc=\$?
		echo \"curl_rc=\$rc http_code=\$code body=\$(cat \"\$tmp\" 2>/dev/null)\"
		rm -f \"\$tmp\"
		echo '--- metrics ---'
		curl --max-time 5 -sS http://127.0.0.1:2379/metrics 2>/dev/null | egrep '^etcd_mvcc_db_total_size_in_bytes |^etcd_server_quota_backend_bytes ' || true
		echo '--- endpoint status ---'
		sudo podman exec etcd-server sh -lc 'export ETCDCTL_API=3; etcdctl --endpoints=http://127.0.0.1:2379 endpoint status -w table' || true
		echo '--- alarms ---'
		sudo podman exec etcd-server sh -lc 'export ETCDCTL_API=3; etcdctl --endpoints=http://127.0.0.1:2379 alarm list' || true
	"
}

set_quota_env() {
	local host="$1"
	local backup_suffix
	backup_suffix="$(date -u +%Y%m%d%H%M%S)"

	log "${host}: backing up /etc/containers/systemd/etcd.container"
	remote "${host}" "sudo cp /etc/containers/systemd/etcd.container /etc/containers/systemd/etcd.container.bak.${backup_suffix}"

	log "${host}: setting ETCD_QUOTA_BACKEND_BYTES=${QUOTA_BYTES}"
	remote "${host}" "
		set -e
		tmp=\$(mktemp)
		sudo awk -v quota='${QUOTA_BYTES}' '
			/^Environment=ETCD_QUOTA_BACKEND_BYTES=/ { next }
			{
				print
				if (\$0 == \"[Container]\" && !inserted) {
					print \"Environment=ETCD_QUOTA_BACKEND_BYTES=\" quota
					inserted = 1
				}
			}
			END {
				if (!inserted) {
					print \"\"
					print \"[Container]\"
					print \"Environment=ETCD_QUOTA_BACKEND_BYTES=\" quota
				}
			}
		' /etc/containers/systemd/etcd.container > \"\$tmp\"
		sudo install -m 0644 \"\$tmp\" /etc/containers/systemd/etcd.container
		rm -f \"\$tmp\"
		sudo systemctl daemon-reload
		echo '--- source quota env with context ---'
		sudo grep -n -B3 -A3 '^Environment=ETCD_QUOTA_BACKEND_BYTES=' /etc/containers/systemd/etcd.container
		echo '--- generated unit quota env ---'
		systemctl cat etcd 2>/dev/null | grep -n 'ETCD_QUOTA_BACKEND_BYTES' || true
	"
}

wait_for_member_after_restart() {
	local host="$1"
	local i quota

	for i in {1..90}; do
		if ! remote "${host}" "systemctl is-active --quiet etcd"; then
			log "${host}: etcd systemd not active yet (${i}/90)"
			sleep 2
			continue
		fi

		quota="$(remote "${host}" "curl --max-time 3 -sS http://127.0.0.1:2379/metrics 2>/dev/null | awk '/^etcd_server_quota_backend_bytes / {print int(\$2); exit}'" || true)"
		if [[ "${quota}" == "${QUOTA_BYTES}" ]]; then
			log "${host}: metrics responsive and quota=${quota}"
			return 0
		fi

		log "${host}: waiting for metrics quota=${QUOTA_BYTES}; saw '${quota:-none}' (${i}/90)"
		remote "${host}" "
			set +e
			tmp=\$(mktemp)
			code=\$(curl --max-time 2 -sS -o \"\$tmp\" -w '%{http_code}' http://127.0.0.1:2379/health 2>&1)
			rc=\$?
			echo \"health curl_rc=\$rc http_code=\$code body=\$(cat \"\$tmp\" 2>/dev/null)\"
			rm -f \"\$tmp\"
			sudo podman ps --filter name=etcd-server --format 'container={{.Names}} status={{.Status}}' || true
			systemctl cat etcd 2>/dev/null | grep 'ETCD_QUOTA_BACKEND_BYTES' || true
			sudo podman inspect etcd-server --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null | grep '^ETCD_QUOTA_BACKEND_BYTES=' || true
			sudo journalctl -u etcd --no-pager -n 8 || true
		" || true
		sleep 2
	done

	log "${host}: failed waiting for quota after restart; final snapshot follows"
	show_node_snapshot "${host}" || true
	return 1
}

log "Increasing etcd backend quota to ${QUOTA_BYTES} bytes on ${#nodes[@]} nodes"
log "Note: /health may return HTTP 503 while NOSPACE alarm is active; this script logs it but waits on systemd plus metrics quota."

log "Pre-change snapshots"
for h in "${nodes[@]}"; do
	echo "===== PRE ${h} ====="
	show_node_snapshot "${h}" || true
done

log "Phase 1: update Quadlet source on all nodes"
for h in "${nodes[@]}"; do
	echo "===== CONFIG ${h} ====="
	set_quota_env "${h}"
done

log "Phase 2: rolling restart one member at a time"
for h in "${nodes[@]}"; do
	echo "===== RESTART ${h} ====="
	log "${h}: restarting etcd"
	remote "${h}" "sudo systemctl restart etcd"
	wait_for_member_after_restart "${h}"
	log "${h}: restart complete"
	show_node_snapshot "${h}" || true
done

log "Phase 3: disarm NOSPACE alarms"
remote_etcdctl "${nodes[0]}" "alarm disarm" || true

log "Post-change snapshots"
for h in "${nodes[@]}"; do
	echo "===== POST ${h} ====="
	show_node_snapshot "${h}" || true
done

log "Done"
