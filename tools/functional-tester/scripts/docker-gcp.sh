#!/usr/bin/env bash
set -e

<<COMMENT
GCP_KEY_PATH=/etc/gcp-key-etcd-development.json \
  ./scripts/docker-gcp.sh

GCP_KEY_PATH=/etc/gcp-key-etcd-development.json \
  FAILPOINTS=1 \
  ./scripts/docker-gcp.sh

gcloud compute ssh --zone us-west1-a functional-tester
gcloud compute ssh --zone us-west1-a functional-tester-failpoints

tail -f /tmp/ansible.log

sudo journalctl -u etcd-agent-1.service -l --no-pager|less
sudo journalctl -u etcd-agent-2.service -l --no-pager|less
sudo journalctl -u etcd-agent-3.service -l --no-pager|less

sudo journalctl -u etcd-tester.service -l --no-pager|less
sudo journalctl -f -u etcd-tester.service

sudo systemctl disable etcd-tester.service
sudo systemctl stop etcd-tester.service
COMMENT

if ! [[ "${0}" =~ "scripts/docker-gcp.sh" ]]; then
  echo "must be run from tools/functional-tester"
  exit 255
fi

if [[ "${GCP_KEY_PATH}" ]]; then
  echo GCP_KEY_PATH is defined: \""${GCP_KEY_PATH}"\"
else
  echo GCP_KEY_PATH is not defined!
  exit 255
fi

if [[ "${FAILPOINTS}" ]]; then
  INSTANCE_NAME=functional-tester-failpoints
  ANSIBLE_SCRIPT=scripts/docker-gcp-failpoints.ansible.sh
else
  INSTANCE_NAME=functional-tester
  ANSIBLE_SCRIPT=scripts/docker-gcp-normal.ansible.sh
fi
echo "INSTANCE_NAME:" ${INSTANCE_NAME}
echo "ANSIBLE_SCRIPT:" ${ANSIBLE_SCRIPT}

gcloud compute instances create \
  ${INSTANCE_NAME} \
  --custom-cpu=10 \
  --custom-memory=40 \
  --zone us-west1-a \
  --image-family=ubuntu-1604-lts \
  --image-project=ubuntu-os-cloud \
  --boot-disk-size=200 \
  --boot-disk-type="pd-ssd" \
  --network default-new \
  --maintenance-policy=MIGRATE \
  --restart-on-failure \
  --metadata-from-file gcp-key-etcd=${GCP_KEY_PATH},startup-script=${ANSIBLE_SCRIPT}
