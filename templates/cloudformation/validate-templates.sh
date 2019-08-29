#!/bin/bash

# Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

set -e

function validate {
    echo "validating $1"

    aws cloudformation validate-template --template-body "file://$1"

    echo "looks good!"
}

for t in *.yml
do
    validate $t
done
