#!/usr/bin/env bash
set -euo pipefail

npx live-server --host=127.0.0.1 --port=5500 --open=index.html ./static

