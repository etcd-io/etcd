#!/bin/sh

echo "Warning: etcd-local-tester is now deprecated in favor of our robustness testing suite and will be removed in a future release."

exec tools/local-tester/bridge/bridge \
  -delay-accept    \
  -reset-listen    \
  -conn-fault-rate=0.25  \
  -immediate-close  \
  -blackhole    \
  -time-close    \
  -write-remote-only  \
  -read-remote-only  \
  -random-blackhole  \
  -corrupt-receive  \
  -corrupt-send    \
  -reorder    \
  $@
