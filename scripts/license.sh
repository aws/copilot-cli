#!/bin/bash
# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
#
# license.sh checks that all Go files in the given correct-looking license header.

check_header() {
    got=$1
    want=$(cat <<EOF
// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
EOF
    )
    if [[ "$got" = *"$want"* ]]; then
        return 0
    fi
    return 1
}

if [ $# != 1 ]; then
    echo "Usage: $0 rootdir" >&2
    exit 1
fi

fail=0
for file in $(find $1 -regextype 'posix-extended' -regex '.*\.(go|js)'); do
    case $file in
        $1/*/mocks/*)
            # Skip mocks packages.
        ;;
        $1/*/mock_*.go)
            # Skip mock files.
        ;;
        $1/*/node_modules/*)
            # Skip node modules for js files.
        ;;
        $1/site/*)
            # Skip website content
        ;;
        *)
            header="$(head -10 $file)"
            if ! check_header "$header"; then
                fail=1
                echo "${file#$1/} doesn't have the right copyright header:"
                echo "$header" | sed -e 's/^/    /g'
            fi
            ;;
    esac
done

if [ $fail -ne 0 ]; then
    exit 1
fi
