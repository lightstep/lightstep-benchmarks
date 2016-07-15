#!/bin/bash

set -e

PROJECT_ID="lightstep-dev"
CLOUD_ZONE="us-central1-a"
MACHINE_TYPE="n1-standard-1"

SSH="gcloud compute ssh --project ${PROJECT_ID} --zone ${CLOUD_ZONE}"

TAG=$(date "+%Y-%m-%d-%H-%M-%S")
INSTANCE="image-build"
IMAGE="client-benchmarks-${TAG}"

yes | gcloud compute instances delete ${INSTANCE} 2> /dev/null || true
yes | gcloud compute disks delete ${INSTANCE} 2> /dev/null || true

gcloud compute instances create ${INSTANCE} \
       --project ${PROJECT_ID} \
       --zone ${CLOUD_ZONE} \
       --machine-type ${MACHINE_TYPE} \
       --image-project ubuntu-os-cloud \
       --image-family ubuntu-1510 \
       --no-boot-disk-auto-delete

# N.B. the image family above (ubuntu-1510) corresponds to the repo
# (ubuntu-wily) listed below.

# Wait for sshd to come up
while true; do
    if ${SSH} ${INSTANCE} true; then
	break
    fi
    sleep 2
done

${SSH} ${INSTANCE} -- sudo bash <<"EOF"
set -e
apt-get update
apt-get install -y apt-transport-https ca-certificates
apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
echo "deb https://apt.dockerproject.org/repo ubuntu-wily main" > /etc/apt/sources.list.d/docker.list
apt-get update
apt-get purge lxc-docker
apt-cache policy docker-engine
apt-get install -y linux-image-extra-$(uname -r)
apt-get install -y docker-engine
service docker start
EOF

yes | gcloud compute instances stop ${INSTANCE} \
  --project ${PROJECT_ID} \
  --zone ${CLOUD_ZONE}

yes | gcloud compute instances delete ${INSTANCE}

gcloud compute images create ${IMAGE} \
  --project ${PROJECT_ID} \
  --family client-benchmarks \
  --source-disk ${INSTANCE} \
  --source-disk-zone ${CLOUD_ZONE}

yes | gcloud compute disks delete ${INSTANCE}

echo Created image ${IMAGE}
