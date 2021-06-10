#!/bin/bash
set -e

usage() {
	echo "run-e2e <test suite>"
	echo "ex: run-e2e addons"
	echo "Runs the specified test suite."
	echo ""
	echo "------------------------    Configuration    -------------------------"
	echo "In addition to the \"test suite\" positional argument, this script"
	echo "is controlled by the following environment variables."
	echo "  DOCKERHUB_USERNAME (Required) The username for the Dockerhub account."
	echo "  DOCKERHUB_TOKEN	   (Required) A Dockerhub API token."
	echo "  AWS_DIR            Full path to directory containing long-lived AWS creds to"
	echo "                       mount into the Docker container."
}

if [[ $# -eq 0 ]]; then
	usage
	exit 0
fi

if [[ -z ${DOCKERHUB_TOKEN} ]]; then
	echo "DOCKERHUB_TOKEN is unset; please export this variable into your enviroment."
	exit 1
fi

if [[ -z ${DOCKERHUB_USERNAME} ]]; then
	echo "DOCKERHUB_USERNAME is unset; please export this variable into your enviroment."
	exit 1
fi

if [[ -z ${AWS_DIR} ]]; then
	AWS_DIR=${HOME}/aws
fi


make build-e2e
echo "Running test suite $@ with credentials sourced from $AWS_DIR and Dockerhub user $DOCKERHUB_USERNAME"
docker run --privileged -v ${AWS_DIR}:/home/.aws -e "HOME=/home" -e "DOCKERHUB_TOKEN=$DOCKERHUB_TOKEN" -e "DOCKERHUB_USERNAME=$DOCKERHUB_USERNAME" -e "TEST_SUITE=$@" copilot/e2e:latest
