#!/bin/bash

#set -x

RATIO_LIST="1/128 1/8 1/4 1/2 2/1 4/1 8/1 128/1"
VALUE_SIZE_POWER_RANGE="8 14"
CONN_CLI_COUNT_POWER_RANGE="5 11"
REPEAT_COUNT=5
RUN_COUNT=200000

KEY_SIZE=256
KEY_SPACE_SIZE=$((1024 * 64))
BACKEND_SIZE="$((20 * 1024 * 1024 * 1024))"
RANGE_RESULT_LIMIT=100
CLIENT_PORT="23790"

COMMIT=

ETCD_ROOT_DIR="$(cd $(dirname $0) && pwd)/../.."
ETCD_BIN_DIR="${ETCD_ROOT_DIR}/bin"
ETCD_BIN="${ETCD_BIN_DIR}/etcd"
ETCD_BM_BIN="${ETCD_ROOT_DIR}/tools/benchmark/benchmark"

WORKING_DIR="$(mktemp -d)"
CURRENT_DIR="$(pwd -P)"
OUTPUT_FILE="${CURRENT_DIR}/result-$(date '+%Y%m%d%H%M').csv"

trap ctrl_c INT

CURRENT_ETCD_PID=

function ctrl_c() {
  # capture ctrl-c and kill server
  echo "terminating..."
  kill_etcd_server ${CURRENT_ETCD_PID}
  exit 0
}

function quit() {
  if [ ! -z ${CURRENT_ETCD_PID} ]; then
    kill_etcd_server ${CURRENT_ETCD_PID}
  fi
  exit $1
}

function check_prerequisite() {
  # check initial parameters
  if [ -f "${OUTPUT_FILE}" ]; then
    echo "file ${OUTPUT_FILE} already exists."
    exit 1
  fi
  pushd ${ETCD_ROOT_DIR} > /dev/null
  COMMIT=$(git log --pretty=format:'%h' -n 1)
  if [ $? -ne 0 ]; then
    COMMIT=N/A
  fi
  popd > /dev/null
  cat >"${OUTPUT_FILE}" <<EOF
type,ratio,conn_size,value_size$(for i in $(seq 1 ${REPEAT_COUNT});do echo -n ",iter$i"; done),comment
PARAM,,,$(for i in $(seq 1 ${REPEAT_COUNT});do echo -n ","; done),"key_size=${KEY_SIZE},key_space_size=${KEY_SPACE_SIZE},backend_size=${BACKEND_SIZE},range_limit=${RANGE_RESULT_LIMIT},commit=${COMMIT}"
EOF

}

function run_etcd_server() {
  if [ ! -x ${ETCD_BIN} ]; then
    echo "no etcd binary found at: ${ETCD_BIN}"
    exit 1
  fi
  # delete existing data directories
  [ -d "db" ] && rm -rf db
  [ -d "default.etcd" ] && rm -rf default.etcd/
  echo "start etcd server in the background"
  ${ETCD_BIN} --quota-backend-bytes=${BACKEND_SIZE} \
    --log-level 'error' \
    --listen-client-urls http://0.0.0.0:${CLIENT_PORT} \
    --advertise-client-urls http://127.0.0.1:${CLIENT_PORT} \
    &>/dev/null &
  return $!
}

function init_etcd_db() {
  #initialize etcd database
  if [ ! -x ${ETCD_BM_BIN} ]; then
    echo "no etcd benchmark binary found at: ${ETCD_BM_BIN}"
    quit -1
  fi
  echo "initialize etcd database..."
  ${ETCD_BM_BIN} put --sequential-keys \
    --key-space-size=${KEY_SPACE_SIZE} \
    --val-size=${VALUE_SIZE} --key-size=${KEY_SIZE} \
    --endpoints http://127.0.0.1:${CLIENT_PORT} \
    --total=${KEY_SPACE_SIZE} \
    &>/dev/null
}

function kill_etcd_server() {
  # kill etcd server
  ETCD_PID=$1
  if [ -z "$(ps aux | grep etcd | awk "{print \$2}")" ]; then
    echo "failed to find the etcd instance to kill: ${ETCD_PID}"
    return
  fi
  echo "kill etcd server instance"
  kill -9 ${ETCD_PID}
  wait ${ETCD_PID} 2>/dev/null
  sleep 5
}


while getopts ":w:c:p:l:vh" OPTION; do
  case $OPTION in
  h)
    echo "usage: $(basename $0) [-h] [-w WORKING_DIR] [-c RUN_COUNT] [-p PORT] [-l RANGE_QUERY_LIMIT] [-v]" >&2
    exit 1
    ;;
  w)
    WORKING_DIR="${OPTARG}"
    ;;
  c)
    RUN_COUNT="${OPTARG}"
    ;;
  p)
    CLIENT_PORT="${OPTARG}"
    ;;
  v)
    set -x
    ;;
  l)
    RANGE_RESULT_LIMIT="${OPTARG}"
    ;;
  \?)
    echo "usage: $(basename $0) [-h] [-w WORKING_DIR] [-c RUN_COUNT] [-p PORT] [-l RANGE_QUERY_LIMIT] [-v]" >&2
    exit 1
    ;;
  esac
done
shift "$((${OPTIND} - 1))"

check_prerequisite

pushd "${WORKING_DIR}" > /dev/null

# progress stats management
ITER_TOTAL=$(($(echo ${RATIO_LIST} | wc | awk "{print \$2}") * \
  $(seq ${VALUE_SIZE_POWER_RANGE} | wc | awk "{print \$2}") * \
  $(seq ${CONN_CLI_COUNT_POWER_RANGE} | wc | awk "{print \$2}")))
ITER_CURRENT=0
PERCENTAGE_LAST_PRINT=0
PERCENTAGE_PRINT_THRESHOLD=5

for RATIO_STR in ${RATIO_LIST}; do
  RATIO=$(echo "scale=4; ${RATIO_STR}" | bc -l)
  for VALUE_SIZE_POWER in $(seq ${VALUE_SIZE_POWER_RANGE}); do
    VALUE_SIZE=$((2 ** ${VALUE_SIZE_POWER}))
    for CONN_CLI_COUNT_POWER in $(seq ${CONN_CLI_COUNT_POWER_RANGE}); do

      # progress stats management
      ITER_CURRENT=$((${ITER_CURRENT} + 1))
      PERCENTAGE_CURRENT=$(echo "scale=3; ${ITER_CURRENT}/${ITER_TOTAL}*100" | bc -l)
      if [ "$(echo "${PERCENTAGE_CURRENT} - ${PERCENTAGE_LAST_PRINT} > ${PERCENTAGE_PRINT_THRESHOLD}" |
        bc -l)" -eq 1 ]; then
        PERCENTAGE_LAST_PRINT=${PERCENTAGE_CURRENT}
        echo "${PERCENTAGE_CURRENT}% completed"
      fi

      CONN_CLI_COUNT=$((2 ** ${CONN_CLI_COUNT_POWER}))

      run_etcd_server
      CURRENT_ETCD_PID=$!
      sleep 5

      init_etcd_db

      START=$(date +%s)
      LINE="DATA,${RATIO},${CONN_CLI_COUNT},${VALUE_SIZE}"
      echo -n "run with setting [${LINE}]"
      for i in $(seq ${REPEAT_COUNT}); do
        echo -n "."
        QPS=$(${ETCD_BM_BIN} txn-mixed "" \
          --conns=${CONN_CLI_COUNT} --clients=${CONN_CLI_COUNT} \
          --total=${RUN_COUNT} \
          --endpoints "http://127.0.0.1:${CLIENT_PORT}" \
          --rw-ratio ${RATIO} --limit ${RANGE_RESULT_LIMIT} \
          2>/dev/null | grep "Requests/sec" | awk "{print \$2}")
        if [ $? -ne 0 ]; then
          echo "benchmark command failed: $?"
          quit -1
        fi
        RD_QPS=$(echo -e "${QPS}" | sed -n '1 p')
        WR_QPS=$(echo -e "${QPS}" | sed -n '2 p')
        if [ -z "${RD_QPS}" ]; then
          RD_QPS=0
        fi
        if [ -z "${WR_QPS}" ]; then
          WR_QPS=0
        fi
        LINE="${LINE},${RD_QPS}:${WR_QPS}"
      done
      END=$(date +%s)
      DIFF=$((${END} - ${START}))
      echo "took ${DIFF} seconds"

      cat >>"${OUTPUT_FILE}" <<EOF
${LINE}
EOF
      kill_etcd_server ${CURRENT_ETCD_PID}
    done
  done
done

popd > /dev/null
