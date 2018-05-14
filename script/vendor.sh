#!/usr/bin/env bash
set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SHED_DIR="$( dirname "$DIR" )"

TMP_WORK_DIR="$( mktemp -d )"

cp -r "${SHED_DIR}/vendor/github.com/Sirupsen/logrus" "${TMP_WORK_DIR}/logrus"
cp -r "${SHED_DIR}/vendor/github.com/docker/docker" "${TMP_WORK_DIR}/docker"

dep ensure

cp -r "${TMP_WORK_DIR}/docker" "${SHED_DIR}/vendor/github.com/docker/docker"
mkdir -p "${SHED_DIR}/vendor/github.com/Sirupsen"
cp -r "${TMP_WORK_DIR}/logrus" "${SHED_DIR}/vendor/github.com/Sirupsen/logrus"

