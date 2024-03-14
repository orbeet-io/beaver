#!/bin/bash
# Local build demo file
# Demonstrate that leaves dependencies should be built before root ones
set -ex

pushd ./hcl2
helm dependency build
popd
pushd ./hcl1
helm dependency build
helm template .
popd
rm hcl*/{charts,Chart.lock} -r
