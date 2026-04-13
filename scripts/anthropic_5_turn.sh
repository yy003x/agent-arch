#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
PERSONA_ID="${PERSONA_ID:-default}"
MODEL="${MODEL:-claude-sonnet-4-5}"

if [[ -z "${ANTHROPIC_BASE_URL:-}" ]]; then
  echo "ANTHROPIC_BASE_URL is required" >&2
  exit 1
fi

if [[ -z "${ANTHROPIC_AUTH_TOKEN:-}" ]]; then
  echo "ANTHROPIC_AUTH_TOKEN is required" >&2
  exit 1
fi

echo "Using server: ${BASE_URL}"
echo "Using Anthropic endpoint: ${ANTHROPIC_BASE_URL}"
echo

create_response="$(curl -sS "${BASE_URL}/v1/agents" \
  -H 'Content-Type: application/json' \
  -d "{\"persona_id\":\"${PERSONA_ID}\",\"provider\":\"anthropic\",\"model\":\"${MODEL}\"}")"

session_id="$(printf '%s' "${create_response}" | sed -n 's/.*"session_id":"\([^"]*\)".*/\1/p')"

if [[ -z "${session_id}" ]]; then
  echo "failed to create agent" >&2
  echo "${create_response}" >&2
  exit 1
fi

echo "Session: ${session_id}"
echo

chat() {
  local turn="$1"
  local message="$2"
  local response

  echo "== ${turn} =="
  echo "User: ${message}"

  response="$(curl -sS "${BASE_URL}/v1/chat" \
    -H 'Content-Type: application/json' \
    -d "{\"session_id\":\"${session_id}\",\"message\":\"${message}\"}")"

  echo "Assistant JSON: ${response}"
  echo
}

chat "Round 1" "第1轮：请记住，我最喜欢的编程语言是 Go。"
chat "Round 2" "第2轮：我们聊聊 HTTP API 设计。"
chat "Round 3" "第3轮：再聊聊上下文裁剪策略。"
chat "Round 4" "第4轮：总结一下前面的实现约束。"
chat "Round 5" "第5轮：回答第1轮的问题，我最喜欢的编程语言是什么？"

echo "== Session Memory =="
curl -sS "${BASE_URL}/v1/sessions/${session_id}/memory"
echo
