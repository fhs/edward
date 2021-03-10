#!/bin/bash

mkdir -p /tmp/testedward
export NAMESPACE=/tmp/testedward
./edward -validateboxes $*

