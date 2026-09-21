#!/usr/bin/env bash
# Sends one Choice question to the real jev API and prints the raw response and timing.
set -euo pipefail
: "${TYPESAFE_API_KEY:?set TYPESAFE_API_KEY first}"
endpoint="${JEV_ENDPOINT:-https://api.typesafe.ai/v1/systemone}"

curl -sS -w '\n--- http=%{http_code} time_total=%{time_total}s\n' "$endpoint" \
  -H "Authorization: Bearer ${TYPESAFE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "jev-latest",
    "state": {"board": [[2,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0]]},
    "questions": {
      "move": {
        "type": "choice",
        "instructions": "You are playing 2048 on a 4x4 board (0 = empty). Choose the best move.",
        "criteria": {
          "down": "Slide all tiles toward the bottom row.",
          "right": "Slide all tiles toward the rightmost column."
        }
      }
    }
  }'
