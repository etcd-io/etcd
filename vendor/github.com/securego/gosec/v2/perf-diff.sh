#!/bin/bash

BIN="gosec"
BUILD_DIR="/tmp/securego"

# Scan the current folder and measure the duration.
function scan() {
  local scan_cmd=$1
  s=$(date +%s%3N)
  $scan_cmd -quiet ./...
  e=$(date +%s%3N)
  res=$(expr $e - $s)
  echo $res
}

# Build the master reference version.
mkdir -p ${BUILD_DIR}
git clone --quiet https://github.com/securego/gosec.git ${BUILD_DIR} >/dev/null
make -C ${BUILD_DIR} >/dev/null

# Scan once with the main reference.
duration_master=$(scan "${BUILD_DIR}/${BIN}")
echo "gosec reference time: ${duration_master}ms"

# Build the current version.
make -C . >/dev/null

# Scan once with the current version.
duration=$(scan "./${BIN}")
echo "gosec time: ${duration}ms"

# Compute the difference of the execution time.
diff=$(($duration - $duration_master))
if [[ diff -lt 0 ]]; then
  diff=$(($diff * -1))
fi
echo "diff: ${diff}ms"
perf=$((100 - ($duration * 100) / $duration_master))
echo "perf diff: ${perf}%"

# Fail the build if there is a performance degradation of more than 10%.
if [[ $perf -lt -10 ]]; then
  exit 1
fi
