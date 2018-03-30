#!/usr/bin/env bash
set -e

echo "root ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

##########################################################
apt-get -y --allow-unauthenticated install ansible

cat > /tmp/ansible.yml <<EOF
---
- name: a play that runs entirely on the ansible host
  hosts: localhost
  connection: local

  tasks:
  - file:
      path: /usr/local/bin
      state: directory
      mode: 0755

  - file:
      path: /var/log/etcd-log
      state: directory
      mode: 0755

  - name: Download Docker installer
    get_url:
      url=https://get.docker.com
      dest=/tmp/docker.sh

  - name: Execute the docker.sh
    script: /tmp/docker.sh
EOF

ansible-playbook /tmp/ansible.yml > /tmp/ansible.log 2>&1
##########################################################

##########################################################
cat > /tmp/etcd-agent-1.service <<EOF
[Unit]
Description=etcd

[Service]
Restart=on-failure
RestartSec=5s
TimeoutStartSec=0
LimitNOFILE=40000

ExecStartPre=/usr/bin/docker pull gcr.io/etcd-development/etcd-functional-tester:latest
ExecStartPre=/bin/mkdir -p /var/log/etcd-log/agent-1
ExecStart=/usr/bin/docker run \
    --rm \
    --net=host \
    --name agent-1 \
    --mount type=bind,source=/var/log/etcd-log/agent-1,destination=/agent-1 \
    gcr.io/etcd-development/etcd-functional-tester:go1.9.4 \
    /bin/bash -c "/etcd-agent \
      --etcd-path /etcd \
      --etcd-log-dir /agent-1 \
      --port :19027 \
      --failpoint-addr :7381"

[Install]
WantedBy=multi-user.target
EOF
cat /tmp/etcd-agent-1.service
mv -f /tmp/etcd-agent-1.service /etc/systemd/system/etcd-agent-1.service

cat > /tmp/etcd-agent-2.service <<EOF
[Unit]
Description=etcd

[Service]
Restart=on-failure
RestartSec=5s
TimeoutStartSec=0
LimitNOFILE=40000

ExecStartPre=/usr/bin/docker pull gcr.io/etcd-development/etcd-functional-tester:latest
ExecStartPre=/bin/mkdir -p /var/log/etcd-log/agent-2
ExecStart=/usr/bin/docker run \
    --rm \
    --net=host \
    --name agent-2 \
    --mount type=bind,source=/var/log/etcd-log/agent-2,destination=/agent-2 \
    gcr.io/etcd-development/etcd-functional-tester:go1.9.4 \
    /bin/bash -c "/etcd-agent \
      --etcd-path /etcd \
      --etcd-log-dir /agent-2 \
      --port :29027 \
      --failpoint-addr :7382"

[Install]
WantedBy=multi-user.target
EOF
cat /tmp/etcd-agent-2.service
mv -f /tmp/etcd-agent-2.service /etc/systemd/system/etcd-agent-2.service

cat > /tmp/etcd-agent-3.service <<EOF
[Unit]
Description=etcd

[Service]
Restart=on-failure
RestartSec=5s
TimeoutStartSec=0
LimitNOFILE=40000

ExecStartPre=/usr/bin/docker pull gcr.io/etcd-development/etcd-functional-tester:latest
ExecStartPre=/bin/mkdir -p /var/log/etcd-log/agent-3
ExecStart=/usr/bin/docker run \
    --rm \
    --net=host \
    --name agent-3 \
    --mount type=bind,source=/var/log/etcd-log/agent-3,destination=/agent-3 \
    gcr.io/etcd-development/etcd-functional-tester:go1.9.4 \
    /bin/bash -c "/etcd-agent \
      --etcd-path /etcd \
      --etcd-log-dir /agent-3 \
      --port :39027 \
      --failpoint-addr :7383"

[Install]
WantedBy=multi-user.target
EOF
cat /tmp/etcd-agent-3.service
mv -f /tmp/etcd-agent-3.service /etc/systemd/system/etcd-agent-3.service

cat > /tmp/etcd-tester.service <<EOF
[Unit]
Description=etcd

[Service]
Restart=on-failure
RestartSec=5s
TimeoutStartSec=0
LimitNOFILE=40000

ExecStartPre=/usr/bin/docker pull gcr.io/etcd-development/etcd-functional-tester:latest
ExecStart=/usr/bin/docker run \
  --rm \
  --net=host \
  --name tester \
  gcr.io/etcd-development/etcd-functional-tester:go1.9.4 \
  /bin/bash -c "/etcd-tester \
    --agent-endpoints '127.0.0.1:19027,127.0.0.1:29027,127.0.0.1:39027' \
    --client-ports 1379,2379,3379 \
    --advertise-client-ports 13790,23790,33790 \
    --peer-ports 1380,2380,3380 \
    --advertise-peer-ports 13800,23800,33800 \
    --etcd-runner /etcd-runner \
    --stress-qps=2500 \
    --stress-key-txn-count 100 \
    --stress-key-txn-ops 10 \
    --exit-on-failure"

[Install]
WantedBy=multi-user.target
EOF
cat /tmp/etcd-tester.service
mv -f /tmp/etcd-tester.service /etc/systemd/system/etcd-tester.service
##########################################################

##########################################################
systemctl daemon-reload

systemctl enable etcd-agent-1.service
systemctl start etcd-agent-1.service

sleep 3s
systemctl enable etcd-agent-2.service
systemctl start etcd-agent-2.service

sleep 3s
systemctl enable etcd-agent-3.service
systemctl start etcd-agent-3.service

sleep 3s
systemctl enable etcd-tester.service
systemctl start etcd-tester.service
##########################################################
