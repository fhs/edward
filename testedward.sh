#!/bin/bash

mkdir -p /tmp/testedward
export NAMESPACE=/tmp/testedward
9 plumber &
./edward -validateboxes -debug localhost:2021 $*

