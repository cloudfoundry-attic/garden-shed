#!/usr/bin/env bash
set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SHED_DIR="$( dirname "$DIR" )"

TMP_WORK_DIR="$( mktemp -d )"

function backup_manual_vends() {
  src="$1"
  dst="$2"

  cp -r "${src}/vendor/github.com/Sirupsen/logrus" "${dst}"
  cp -r "${src}/vendor/github.com/docker/docker" "${dst}"
}

function restore_manual_vends() {
  src="$1"
  dst="$2"

  mkdir -p "${src}/vendor/github.com/docker"
  mkdir -p "${src}/vendor/github.com/Sirupsen"

  cp -r "${dst}/docker" "${src}/vendor/github.com/docker"
  cp -r "${dst}/logrus" "${src}/vendor/github.com/Sirupsen"
}

backup_manual_vends "$SHED_DIR" "$TMP_WORK_DIR"
trap "restore_manual_vends ${SHED_DIR} ${TMP_WORK_DIR}" EXIT

dep ensure -v

