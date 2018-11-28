#!/bin/sh

set -e

remote="$1"
url="$2"

.github/assert-contributors.sh

exit 0
