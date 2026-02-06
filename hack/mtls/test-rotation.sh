#!/bin/bash
#
# CA Rotation Test Script
#
# Tests the CA certificate rotation feature with a 4-node cluster:
# 1. Start 3-node cluster (node1, node2, node3) with CA1 certs
# 2. Add CA2 to trust bundle, add node4 with CA2 cert
# 3. Rotate node2 and node3 certs to CA2
# 4. Remove CA1, verify node1 (still on CA1) can't rejoin
#    (nodes 2, 3, 4 maintain quorum with 3 of 4 members)
#
# Usage:
#   ./test-rotation.sh         # Interactive mode (prompts between phases)
#   ./test-rotation.sh -y      # Non-interactive mode (runs all phases automatically)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

ETCDCTL="$SCRIPT_DIR/../../bin/etcdctl"
ETCD="$SCRIPT_DIR/../../bin/etcd"
LOG_DIR="$SCRIPT_DIR/logs"
RELOAD_WAIT=5  # seconds to wait for CA reload (interval is 3s)

# Verify required binaries exist
if [[ ! -x "$ETCD" ]]; then
    echo "ERROR: etcd binary not found at $ETCD" >&2
    echo "Run 'make build' from the repository root first." >&2
    exit 1
fi
if [[ ! -x "$ETCDCTL" ]]; then
    echo "ERROR: etcdctl binary not found at $ETCDCTL" >&2
    exit 1
fi
for cmd in cfssl cfssljson; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: Required command '$cmd' not found in PATH" >&2
        echo "Install with: go install github.com/cloudflare/cfssl/cmd/...@latest" >&2
        exit 1
    fi
done

# Process IDs for each node
NODE1_PID=""
NODE2_PID=""
NODE3_PID=""
NODE4_PID=""

# Parse arguments
INTERACTIVE=true
QUIET=false
while [[ $# -gt 0 ]]; do
    case $1 in
        -y|--yes|--non-interactive)
            INTERACTIVE=false
            shift
            ;;
        -q|--quiet)
            QUIET=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [-y|--yes|--non-interactive] [-q|--quiet]"
            echo ""
            echo "Options:"
            echo "  -y, --yes, --non-interactive  Run all phases without prompting"
            echo "  -q, --quiet                   Suppress etcd log output (logs still saved to $LOG_DIR/)"
            echo "  -h, --help                    Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h for help"
            exit 1
            ;;
    esac
done

# Set up logging - always log to files, clear previous logs
rm -rf "$LOG_DIR"
mkdir -p "$LOG_DIR"
if [[ "$QUIET" == "true" ]]; then
    MAKE_QUIET="-s"
else
    MAKE_QUIET=""
fi
echo "Server logs will be written to: $LOG_DIR/"

# Prompt for confirmation between phases (if interactive)
wait_for_continue() {
    if [[ "$INTERACTIVE" == "true" ]]; then
        echo ""
        read -r -p "Press Enter to continue to the next phase (or Ctrl+C to abort)..."
        echo ""
    fi
}

CLEANUP_DONE=false
cleanup() {
    [[ "$CLEANUP_DONE" == "true" ]] && return
    CLEANUP_DONE=true

    echo "Cleaning up..."
    # Kill all tracked processes
    local pids=("$NODE1_PID" "$NODE2_PID" "$NODE3_PID" "$NODE4_PID")
    for pid in "${pids[@]}"; do
        [[ -z "$pid" ]] && continue
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
        fi
    done
    # Wait for processes to terminate
    for pid in "${pids[@]}"; do
        [[ -z "$pid" ]] && continue
        wait "$pid" 2>/dev/null || true
    done
    # Only remove data directories in the current working directory
    if [[ "$PWD" == *"hack/mtls"* ]]; then
        rm -rf ./*.etcd
    fi
    echo "Logs saved to: $LOG_DIR/"
}
trap cleanup EXIT INT TERM

# Start an etcd node and return its PID
start_node() {
    local name=$1
    local client_port=$2
    local peer_port=$3
    local cert_prefix=$4
    local initial_cluster=$5
    local cluster_state=${6:-new}
    local log_file="$LOG_DIR/${name}.log"

    $ETCD --name "$name" \
        --listen-client-urls "https://localhost:$client_port" \
        --advertise-client-urls "https://localhost:$client_port" \
        --listen-peer-urls "https://localhost:$peer_port" \
        --initial-advertise-peer-urls "https://localhost:$peer_port" \
        --initial-cluster-token etcd-cluster-1 \
        --initial-cluster "$initial_cluster" \
        --initial-cluster-state "$cluster_state" \
        --cert-file="certs/${cert_prefix}.pem" \
        --key-file="certs/${cert_prefix}-key.pem" \
        --trusted-ca-file=certs/ca.pem \
        --client-cert-auth \
        --peer-cert-file="certs/${cert_prefix}.pem" \
        --peer-key-file="certs/${cert_prefix}-key.pem" \
        --peer-trusted-ca-file=certs/ca.pem \
        --peer-client-cert-auth \
        --peer-tls-reload-ca \
        --client-tls-reload-ca \
        --tls-ca-reload-interval=3s >> "$log_file" 2>&1 &

    local pid=$!
    sleep 0.5
    if ! kill -0 "$pid" 2>/dev/null; then
        echo "ERROR: Failed to start etcd node $name (check $log_file)" >&2
        return 1
    fi
    echo "$pid"
}

etcdctl_cmd() {
    $ETCDCTL --endpoints=https://localhost:2379,https://localhost:12379,https://localhost:22379 \
        --cacert=certs/ca.pem \
        --cert=certs/client.pem \
        --key=certs/client-key.pem \
        "$@"
}

# Check health of a single endpoint with retries
check_node_health() {
    local endpoint="$1"
    local max_attempts="${2:-3}"
    local attempt=1
    while [[ "$attempt" -le "$max_attempts" ]]; do
        if $ETCDCTL --endpoints="$endpoint" \
            --cacert=certs/ca.pem \
            --cert=certs/client.pem \
            --key=certs/client-key.pem \
            endpoint health -w table 2>&1; then
            return 0
        fi
        echo "Health check attempt $attempt/$max_attempts failed, retrying in 2s..."
        sleep 2
        attempt=$((attempt + 1))
    done
    return 1
}

# Check health of entire cluster (uses --cluster flag)
check_cluster_health() {
    local entry_endpoint="${1:-https://localhost:12379}"
    $ETCDCTL --endpoints="$entry_endpoint" \
        --cacert=certs/ca.pem \
        --cert=certs/client.pem \
        --key=certs/client-key.pem \
        endpoint health --cluster -w table
}

INITIAL_CLUSTER_3='infra1=https://localhost:2380,infra2=https://localhost:12380,infra3=https://localhost:22380'
INITIAL_CLUSTER_4='infra1=https://localhost:2380,infra2=https://localhost:12380,infra3=https://localhost:22380,infra4=https://localhost:32380'

echo "=========================================="
echo "  CA Rotation Test (4-node cluster)"
echo "=========================================="
if [[ "$INTERACTIVE" == "true" ]]; then
    echo "Running in INTERACTIVE mode (use -y for non-interactive)"
else
    echo "Running in NON-INTERACTIVE mode"
fi
echo ""

echo "=== Phase 1: Initial 3-node cluster with CA1 ==="
echo "Starting nodes 1, 2, 3 - all with CA1 certificates."
echo ""
make $MAKE_QUIET clean
make $MAKE_QUIET ca1 certs-ca1

NODE1_PID=$(start_node infra1 2379 2380 etcd1 "$INITIAL_CLUSTER_3" new)
NODE2_PID=$(start_node infra2 12379 12380 etcd2 "$INITIAL_CLUSTER_3" new)
NODE3_PID=$(start_node infra3 22379 22380 etcd3 "$INITIAL_CLUSTER_3" new)
echo "Started nodes: node1=$NODE1_PID, node2=$NODE2_PID, node3=$NODE3_PID"
sleep 5

echo "Verifying cluster health..."
etcdctl_cmd member list -w table
check_cluster_health
etcdctl_cmd put phase1 "success"
echo ""
echo ">>> Phase 1 PASSED: 3-node cluster running with CA1 certificates"

wait_for_continue

echo ""
echo "=== Phase 2: Add CA2 and node4 ==="
echo "This phase:"
echo "  1. Creates a new CA (CA2)"
echo "  2. Updates trust bundle to include both CA1 and CA2"
echo "  3. Waits for nodes to reload the new trust bundle"
echo "  4. Adds node4 with a certificate signed by CA2"
echo ""
make $MAKE_QUIET ca2 certs-ca2-node4
cat certs/ca1.pem certs/ca2.pem > certs/ca.pem
echo "Trust bundle updated: ca.pem now contains [CA1, CA2]"
echo "Waiting ${RELOAD_WAIT}s for CA reload..."
sleep $RELOAD_WAIT
echo "Adding node4 to cluster..."
etcdctl_cmd member add infra4 --peer-urls=https://localhost:32380
NODE4_PID=$(start_node infra4 32379 32380 etcd4 "$INITIAL_CLUSTER_4" existing)
echo "Started node4=$NODE4_PID"
sleep 5

echo "Verifying cluster health with 4 nodes..."
etcdctl_cmd member list -w table
check_cluster_health
etcdctl_cmd put phase2 "success"
echo ""
echo ">>> Phase 2 PASSED: node4 (CA2 cert) successfully joined the cluster"

wait_for_continue

echo ""
echo "=== Phase 3: Rotate node2 and node3 to CA2 ==="
echo "This phase rotates node2 and node3 certificates from CA1 to CA2."
echo "Each node will be restarted with its new certificate."
echo ""

# Rotate node2
make $MAKE_QUIET certs-ca2-node2
echo "Generated new certificate for node2 signed by CA2"
echo "Restarting node2..."
kill "$NODE2_PID" 2>/dev/null || true
sleep 2
NODE2_PID=$(start_node infra2 12379 12380 etcd2 "$INITIAL_CLUSTER_4" existing)
echo "Restarted node2=$NODE2_PID"
sleep 5
echo "Checking node2 is up..."
check_node_health "https://localhost:12379" || { echo "Node2 failed to restart"; exit 1; }

# Rotate node3
make $MAKE_QUIET certs-ca2-node3
echo "Generated new certificate for node3 signed by CA2"
echo "Restarting node3..."
kill "$NODE3_PID" 2>/dev/null || true
sleep 2
NODE3_PID=$(start_node infra3 22379 22380 etcd3 "$INITIAL_CLUSTER_4" existing)
echo "Restarted node3=$NODE3_PID"
sleep 5
echo "Checking node3 is up..."
check_node_health "https://localhost:22379" || { echo "Node3 failed to restart"; exit 1; }

echo "Verifying cluster health after rotations..."
etcdctl_cmd member list -w table
check_cluster_health
etcdctl_cmd put phase3 "success"
echo ""
echo ">>> Phase 3 PASSED: node2 and node3 successfully rejoined with CA2 certificates"
echo "    Current state: node1=CA1, node2=CA2, node3=CA2, node4=CA2"

wait_for_continue

echo ""
echo "=== Phase 4: Remove CA1, node1 should fail ==="
echo "This phase removes CA1 from the trust bundle, leaving only CA2."
echo "Node1 (still using CA1 certificate) should fail to communicate"
echo "with nodes 2, 3, and 4. Nodes 2, 3, 4 maintain quorum (3 of 4)."
echo ""

# First, rotate client cert to CA2 so we can still authenticate
echo "Rotating client certificate to CA2..."
make $MAKE_QUIET certs-ca2-client

# Remove CA1 from trust bundle - node1 will become isolated
cp certs/ca2.pem certs/ca.pem
echo "Trust bundle updated: ca.pem now contains only [CA2]"
echo "Waiting ${RELOAD_WAIT}s for CA reload..."
sleep $RELOAD_WAIT

# After reload, nodes 2, 3, 4 only trust CA2
# Node1's CA1 cert will be rejected by nodes 2, 3, 4
# But nodes 2, 3, 4 can still communicate (all have CA2 certs)
echo "Checking health of nodes 2, 3, 4 (should be healthy)..."
check_node_health "https://localhost:12379" || { echo "Phase 4 FAILED: node2 should be healthy"; exit 1; }
check_node_health "https://localhost:22379" || { echo "Phase 4 FAILED: node3 should be healthy"; exit 1; }
check_node_health "https://localhost:32379" || { echo "Phase 4 FAILED: node4 should be healthy"; exit 1; }
echo "Nodes 2, 3, 4 are healthy as expected"

echo ""
echo "Cluster state (from node2's perspective):"
$ETCDCTL --endpoints=https://localhost:12379 \
    --cacert=certs/ca.pem \
    --cert=certs/client.pem \
    --key=certs/client-key.pem \
    member list -w table

# Verify we can still read/write data
echo "Verifying cluster operations on nodes 2, 3, 4..."
$ETCDCTL --endpoints=https://localhost:12379 \
    --cacert=certs/ca.pem \
    --cert=certs/client.pem \
    --key=certs/client-key.pem \
    put phase4 "success"
$ETCDCTL --endpoints=https://localhost:32379 \
    --cacert=certs/ca.pem \
    --cert=certs/client.pem \
    --key=certs/client-key.pem \
    get phase4

# Force node1 to reconnect by restarting it
echo ""
echo "Restarting node1 to force new connections..."
kill "$NODE1_PID" 2>/dev/null || true
sleep 2
NODE1_PID=$(start_node infra1 2379 2380 etcd1 "$INITIAL_CLUSTER_4" existing)
echo "Restarted node1=$NODE1_PID (should fail to peer)"
sleep 5

# Node 1 should fail - its CA1 cert is not trusted by nodes 2, 3, 4
echo "Checking node1 (should be unhealthy - its CA1 cert is rejected by other nodes)..."
if $ETCDCTL --endpoints=https://localhost:2379 \
    --cacert=certs/ca.pem \
    --cert=certs/client.pem \
    --key=certs/client-key.pem \
    endpoint health -w table 2>&1; then
    echo ""
    echo "Phase 4 FAILED: node1 should not be healthy (its CA1 cert should be rejected)"
    exit 1
fi
echo ""
echo ">>> Phase 4 PASSED: node1 correctly rejected (CA1 cert no longer trusted)"

echo ""
echo "Final cluster state (from node2's perspective):"
$ETCDCTL --endpoints=https://localhost:12379 \
    --cacert=certs/ca.pem \
    --cert=certs/client.pem \
    --key=certs/client-key.pem \
    member list -w table

echo ""
echo "Endpoint health (nodes 2, 3, 4 should be healthy, node1 unhealthy):"
check_cluster_health || true

wait_for_continue

echo ""
echo "=== Phase 5: Add CA1 back, node1 should auto-recover ==="
echo "This phase adds CA1 back to the trust bundle."
echo "Node1 should automatically recover WITHOUT restart."
echo ""

cat certs/ca1.pem certs/ca2.pem > certs/ca.pem
echo "Trust bundle updated: ca.pem now contains [CA1, CA2]"
echo "Waiting ${RELOAD_WAIT}s for CA reload..."
sleep $RELOAD_WAIT

echo "Checking if node1 auto-recovered (may need extra time for reconnection)..."
# Give node1 extra time to reconnect - it may be in backoff
if check_node_health "https://localhost:2379" 5; then
    echo ""
    echo ">>> Phase 5 PASSED: node1 auto-recovered after CA1 was added back"
else
    echo ""
    echo "Phase 5 FAILED: node1 did not recover"
    exit 1
fi

echo ""
echo "Final cluster health (all 4 nodes should be healthy):"
check_cluster_health

echo ""
echo "=========================================="
echo "  ALL CA ROTATION TESTS PASSED"
echo "=========================================="
echo ""
echo "Summary:"
echo "  Phase 1: Started 3-node cluster with CA1"
echo "  Phase 2: Added CA2 to trust bundle, node4 (CA2) joined successfully"
echo "  Phase 3: Rotated node2 and node3 to CA2, both rejoined successfully"
echo "  Phase 4: Removed CA1 from trust bundle"
echo "           - Node1 (CA1 cert) became isolated (rejected by other nodes)"
echo "           - Nodes 2, 3, 4 (CA2 certs) maintained quorum"
echo "  Phase 5: Added CA1 back to trust bundle"
echo "           - Node1 auto-recovered without restart"
echo ""
echo "Stopping all etcd nodes..."
# cleanup is also called via trap EXIT, but we call it explicitly for clear output
cleanup
