#!/bin/bash
# vim: set ft=sh

set -ex

source $(dirname $0)/aufs
cd $(dirname $0)/../../..

rm -rf garden-runc-release-git/src/code.cloudfoundry.org/garden-shed
cp -r garden-shed-git garden-runc-release-git/src/code.cloudfoundry.org/garden-shed

cd garden-runc-release-git
export GOROOT=/usr/local/go
export PATH=$GOROOT/bin:$PATH

export GOPATH=$PWD
export PATH=$GOPATH/bin:$PATH

cd src/code.cloudfoundry.org/garden-shed

permit_device_control
create_loop_devices 256

go tool vet -printf=false .
ginkgo -tags daemon -r -p -race -keepGoing -nodes=8 -failOnPending -randomizeSuites "$@"
