#!/bin/sh
set -e
echo "e2e success: job running"
curl http://front-end.$COPILOT_SERVICE_DISCOVERY_ENDPOINT/oraoraora-setter/
