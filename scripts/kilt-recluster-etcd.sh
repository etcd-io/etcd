#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/kilt-recluster-etcd.sh --apply
  scripts/kilt-recluster-etcd.sh --dry-run
  scripts/kilt-recluster-etcd.sh --verify-only

Bootstraps the five kilt PHX etcd nodes as one fresh 5-member cluster.

By default this runs in dry-run mode. Use --apply to edit the remote
/etc/containers/systemd/etcd.container files, move existing /var/data/etcd
aside, restart etcd, and verify the cluster.

Options:
  --apply             Make changes on the five remote nodes.
  --dry-run           Print the planned node configuration only. Default.
  --verify-only       Only run etcdctl checks from node 1's etcd container.
  --no-reset-data     Do not move /var/data/etcd aside before starting.
  --skip-firewall     Do not run firewall-cmd on the remote nodes.
  --ssh-user USER     SSH user for all nodes. Default: opc.
  --quota-bytes BYTES Set ETCD_QUOTA_BACKEND_BYTES. Default: 8589934592.
  -h, --help          Show this help.
USAGE
}

SSH_USER="opc"
APPLY=0
VERIFY_ONLY=0
RESET_DATA=1
SKIP_FIREWALL=0
QUOTA_BYTES=8589934592

TOKEN="kilt-phx-dev-1-etcd-20260512"
CLUSTER="kilt-phx-dev-1-etcdn1-1=http://10.201.162.151:2380,kilt-phx-dev-1-etcdn2-1=http://10.201.47.57:2380,kilt-phx-dev-1-etcdn3-1=http://10.201.98.50:2380,kilt-phx-dev-1-etcdn4-1=http://10.201.5.177:2380,kilt-phx-dev-1-etcdn5-1=http://10.201.117.10:2380"
ENDPOINTS="http://10.201.162.151:2379,http://10.201.47.57:2379,http://10.201.98.50:2379,http://10.201.5.177:2379,http://10.201.117.10:2379"

NODES=(
  "kilt-phx-dev-1-etcdn1-1 10.201.162.151"
  "kilt-phx-dev-1-etcdn2-1 10.201.47.57"
  "kilt-phx-dev-1-etcdn3-1 10.201.98.50"
  "kilt-phx-dev-1-etcdn4-1 10.201.5.177"
  "kilt-phx-dev-1-etcdn5-1 10.201.117.10"
)

while (($#)); do
  case "$1" in
    --apply)
      APPLY=1
      ;;
    --dry-run)
      APPLY=0
      ;;
    --verify-only)
      VERIFY_ONLY=1
      ;;
    --no-reset-data)
      RESET_DATA=0
      ;;
    --skip-firewall)
      SKIP_FIREWALL=1
      ;;
    --ssh-user)
      shift
      if (($# == 0)); then
        echo "missing value for --ssh-user" >&2
        exit 2
      fi
      SSH_USER="$1"
      ;;
    --quota-bytes)
      shift
      if (($# == 0)); then
        echo "missing value for --quota-bytes" >&2
        exit 2
      fi
      QUOTA_BYTES="$1"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

ssh_target() {
  local target="$1"
  printf '%s@%s' "${SSH_USER}" "${target}"
}

print_plan() {
  echo "Cluster token: ${TOKEN}"
  echo "Cluster map:   ${CLUSTER}"
  echo "Quota bytes:   ${QUOTA_BYTES}"
  echo
  for node in "${NODES[@]}"; do
    set -- ${node}
    local host="$1"
    local ip="$2"
    cat <<PLAN
${host}
  ETCD_NAME=${host}
  ETCD_INITIAL_ADVERTISE_PEER_URLS=http://${ip}:2380
  ETCD_LISTEN_PEER_URLS=http://${ip}:2380
  ETCD_ADVERTISE_CLIENT_URLS=http://${ip}:2379
  ETCD_LISTEN_CLIENT_URLS=http://${ip}:2379,http://127.0.0.1:2379
  ETCD_INITIAL_CLUSTER=${CLUSTER}
  ETCD_INITIAL_CLUSTER_STATE=new
  ETCD_INITIAL_CLUSTER_TOKEN=${TOKEN}

PLAN
  done
}

prepare_node() {
  local host="$1"
  local ip="$2"

  echo "== preparing ${host} (${ip}) =="
  ssh "$(ssh_target "${ip}")" \
    "sudo env NODE_NAME='${host}' NODE_IP='${ip}' CLUSTER='${CLUSTER}' TOKEN='${TOKEN}' RESET_DATA='${RESET_DATA}' SKIP_FIREWALL='${SKIP_FIREWALL}' QUOTA_BYTES='${QUOTA_BYTES}' bash -s" <<'REMOTE'
set -euo pipefail

cfg=/etc/containers/systemd/etcd.container
if [ ! -f "${cfg}" ]; then
  echo "missing ${cfg}" >&2
  exit 1
fi

backup="${cfg}.bak.$(date +%Y%m%d%H%M%S)"
cp -a "${cfg}" "${backup}"
echo "backed up ${cfg} to ${backup}"

set_env() {
  local key="$1"
  local value="$2"
  if grep -q "^Environment=${key}=" "${cfg}"; then
    sed -i "s|^Environment=${key}=.*|Environment=${key}=${value}|" "${cfg}"
  else
    sed -i "/^Notify=/i Environment=${key}=${value}" "${cfg}"
  fi
}

set_env ETCD_NAME "${NODE_NAME}"
set_env ETCD_DATA_DIR "/var/data/etcd"
set_env ETCD_INITIAL_ADVERTISE_PEER_URLS "http://${NODE_IP}:2380"
set_env ETCD_LISTEN_PEER_URLS "http://${NODE_IP}:2380"
set_env ETCD_ADVERTISE_CLIENT_URLS "http://${NODE_IP}:2379"
set_env ETCD_LISTEN_CLIENT_URLS "http://${NODE_IP}:2379,http://127.0.0.1:2379"
set_env ETCD_INITIAL_CLUSTER "${CLUSTER}"
set_env ETCD_INITIAL_CLUSTER_STATE "new"
set_env ETCD_INITIAL_CLUSTER_TOKEN "${TOKEN}"
set_env ETCD_QUOTA_BACKEND_BYTES "${QUOTA_BYTES}"

echo "effective ETCD env:"
grep '^Environment=ETCD_' "${cfg}" | sort

systemctl stop etcd || true
podman rm -f etcd-server || true

if [ "${SKIP_FIREWALL}" != "1" ] &&
   command -v firewall-cmd >/dev/null 2>&1 &&
   systemctl is-active --quiet firewalld; then
  firewall-cmd --permanent --add-port=2379/tcp || true
  firewall-cmd --permanent --add-port=2380/tcp || true
  firewall-cmd --reload || true
fi

if [ "${RESET_DATA}" = "1" ]; then
  if [ -d /var/data/etcd ]; then
    mv /var/data/etcd "/var/data/etcd.precluster.$(date +%Y%m%d%H%M%S)"
  fi
  install -d -m 0755 /var/data/etcd
fi

systemctl daemon-reload
REMOTE
}

start_node() {
  local host="$1"
  local ip="$2"
  echo "== starting ${host} (${ip}) =="
  ssh "$(ssh_target "${ip}")" "sudo systemctl start etcd --no-block"
}

show_status() {
  local host="$1"
  local ip="$2"
  echo "== status ${host} (${ip}) =="
  ssh "$(ssh_target "${ip}")" "sudo systemctl is-active etcd; sudo journalctl -u etcd -n 20 --no-pager"
}

verify_cluster() {
  local first_host first_ip
  read -r first_host first_ip <<<"${NODES[0]}"

  echo "== verifying cluster from ${first_host} (${first_ip}) =="
  ssh "$(ssh_target "${first_ip}")" "sudo podman exec etcd-server sh -lc '
set -e
if command -v etcdctl >/dev/null 2>&1; then
  ETCDCTL=etcdctl
elif [ -x /usr/local/bin/etcdctl ]; then
  ETCDCTL=/usr/local/bin/etcdctl
else
  echo etcdctl not found in etcd-server container >&2
  exit 1
fi
\"\$ETCDCTL\" --endpoints=\"${ENDPOINTS}\" member list -w table
\"\$ETCDCTL\" --endpoints=\"${ENDPOINTS}\" endpoint status -w table
\"\$ETCDCTL\" --endpoints=\"${ENDPOINTS}\" endpoint health
'"
}

if ((VERIFY_ONLY)); then
  verify_cluster
  exit 0
fi

if ((APPLY == 0)); then
  print_plan
  echo "Dry run only. Re-run with --apply to update the remote nodes."
  exit 0
fi

print_plan

for node in "${NODES[@]}"; do
  set -- ${node}
  prepare_node "$1" "$2"
done

for node in "${NODES[@]}"; do
  set -- ${node}
  start_node "$1" "$2"
done

sleep 8

for node in "${NODES[@]}"; do
  set -- ${node}
  show_status "$1" "$2"
done

verify_cluster
