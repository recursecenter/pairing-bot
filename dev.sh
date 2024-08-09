#!/usr/bin/env bash

set -euo pipefail

exec gcloud emulators firestore start --host-port="$FIRESTORE_EMULATOR_HOST"
