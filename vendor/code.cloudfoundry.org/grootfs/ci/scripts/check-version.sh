#!/bin/bash
set -e

VERSION=$(cat version/number)

cd grootfs-release-develop/src/code.cloudfoundry.org/grootfs

grep "\"${VERSION}\"" version.go && exit 0

echo "Version mismatch!"
echo "-----------------"
echo ""
echo "Desired version: ${VERSION}"
echo ""
echo "version.go:"
cat version.go
echo ""
echo "Maybe the pipeline is not ready yet"
exit 1

