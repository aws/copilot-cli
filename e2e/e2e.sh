#!/bin/sh
set -e

runtime=$1

if [[ $runtime == "docker" ]]; then
  # Start dockerd
  /usr/local/bin/dockerd \
    --host=unix:///var/run/docker.sock \
    --host=tcp://127.0.0.1:2375 \
    --storage-driver=overlay2 &>/var/log/docker.log &

  # Wait until dockerd is spun up
  tries=0
  d_timeout=60
  until docker info >/dev/null 2>&1; do
    if [ "$tries" -gt "$d_timeout" ]; then
      cat /var/log/docker.log
      echo 'Timed out trying to connect to internal docker host.' >&2
      exit 1
    fi
    tries=$(($tries + 1))
    sleep 1
  done
fi

echo $DOCKERHUB_TOKEN | ${runtime} login --password-stdin -u $DOCKERHUB_USERNAME

#Run all the e2e tests
cd /github.com/aws/copilot-cli/e2e/$TEST_SUITE && /go/bin/ginkgo -v -r
