#!/usr/bin/env bash
# Builds site/public/product/launch.mp4 from the live UI on demo data.
# Needs: node, ffmpeg, python3 with numpy, Inter and Inter Display fonts installed.
# On Linux, set CHROME to a Chromium binary.
set -euo pipefail
cd "$(dirname "$0")"
export SLOW="${SLOW:-8}"
mkdir -p .work
node mock.mjs > .work/mock.log 2>&1 &
mock=$!
trap 'kill $mock 2>/dev/null' EXIT
sleep 1
node record.mjs
kill $mock
python3 music.py
node render.mjs
ffmpeg -y -v error -i .work/silent.mp4 -i .work/music.wav -c:v copy -c:a aac -b:a 192k -shortest \
  -movflags +faststart ../../../../site/public/product/launch.mp4
echo "wrote site/public/product/launch.mp4"
