#!/bin/sh
rm -rf /tmp/m1.data /tmp/m2.data /tmp/m3.data

/etc/init.d/bind9 start

# get rid of hosts so go lookup won't resolve 127.0.0.1 to localhost
cat /dev/null >/etc/hosts

goreman -f /certs-common-name/Procfile start &
sleep 7s

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379 \
  endpoint health --cluster

sleep 2s
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  put abc def

sleep 2s
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  get abc

sleep 1s && printf "\n"
echo "Step 1. creating root role"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  role add root

sleep 1s && printf "\n"
echo "Step 2. granting readwrite 'foo' permission to role 'root'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  role grant-permission root readwrite foo

sleep 1s && printf "\n"
echo "Step 3. getting role 'root'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  role get root

sleep 1s && printf "\n"
echo "Step 4. creating user 'root'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --interactive=false \
  user add root:123

sleep 1s && printf "\n"
echo "Step 5. granting role 'root' to user 'root'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  user grant-role root root

sleep 1s && printf "\n"
echo "Step 6. getting user 'root'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  user get root

sleep 1s && printf "\n"
echo "Step 7. enabling auth"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  auth enable

sleep 1s && printf "\n"
echo "Step 8. writing 'foo' with 'root:123'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=root:123 \
  put foo bar

sleep 1s && printf "\n"
echo "Step 9. writing 'aaa' with 'root:123'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=root:123 \
  put aaa bbb

sleep 1s && printf "\n"
echo "Step 10. writing 'foo' without 'root:123'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  put foo bar

sleep 1s && printf "\n"
echo "Step 11. reading 'foo' with 'root:123'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=root:123 \
  get foo

sleep 1s && printf "\n"
echo "Step 12. reading 'aaa' with 'root:123'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=root:123 \
  get aaa

sleep 1s && printf "\n"
echo "Step 13. creating a new user 'test-common-name:test-pass'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=root:123 \
  --interactive=false \
  user add test-common-name:test-pass

sleep 1s && printf "\n"
echo "Step 14. creating a role 'test-role'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=root:123 \
  role add test-role

sleep 1s && printf "\n"
echo "Step 15. granting readwrite 'aaa' --prefix permission to role 'test-role'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=root:123 \
  role grant-permission test-role readwrite aaa --prefix

sleep 1s && printf "\n"
echo "Step 16. getting role 'test-role'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=root:123 \
  role get test-role

sleep 1s && printf "\n"
echo "Step 17. granting role 'test-role' to user 'test-common-name'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=root:123 \
  user grant-role test-common-name test-role

sleep 1s && printf "\n"
echo "Step 18. writing 'aaa' with 'test-common-name:test-pass'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=test-common-name:test-pass \
  put aaa bbb

sleep 1s && printf "\n"
echo "Step 19. writing 'bbb' with 'test-common-name:test-pass'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=test-common-name:test-pass \
  put bbb bbb

sleep 1s && printf "\n"
echo "Step 20. reading 'aaa' with 'test-common-name:test-pass'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=test-common-name:test-pass \
  get aaa

sleep 1s && printf "\n"
echo "Step 21. reading 'bbb' with 'test-common-name:test-pass'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  --user=test-common-name:test-pass \
  get bbb

sleep 1s && printf "\n"
echo "Step 22. writing 'aaa' with CommonName 'test-common-name'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  put aaa ccc

sleep 1s && printf "\n"
echo "Step 23. reading 'aaa' with CommonName 'test-common-name'"
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name/ca.crt \
  --cert=/certs-common-name/server.crt \
  --key=/certs-common-name/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  get aaa
