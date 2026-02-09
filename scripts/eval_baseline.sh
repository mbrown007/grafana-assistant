#!/usr/bin/env bash
set -euo pipefail

DATASET="tests/evals/dataset.yaml"
API_URL_DEFAULT="http://localhost:8081/api/chat"
API_URL="${ASSISTANT_API_URL:-$API_URL_DEFAULT}"
OUT_DIR="tests/evals/results"
CATEGORY=""
LIMIT=0
TIMEOUT_SECONDS=120
DELAY_MS=150
MOCK_MODE=false
MOCK_CONFIG="${ASSISTANT_EVAL_MOCK_CONFIG:-tests/evals/config.mock.yaml}"
API_URL_EXPLICIT=false
if [[ -n "${ASSISTANT_API_URL:-}" ]]; then
  API_URL_EXPLICIT=true
fi

tmp_dir=""
MOCK_ASSISTANT_PID=""
MOCK_ASSISTANT_STARTED=false

# Auth options:
# - ASSISTANT_COOKIE: raw Cookie header value from a logged-in Grafana session.
# - ASSISTANT_AUTH_HEADER: full header line, e.g. "Authorization: Bearer <token>".
COOKIE_HEADER="${ASSISTANT_COOKIE:-}"
AUTH_HEADER="${ASSISTANT_AUTH_HEADER:-}"

usage() {
  cat <<'EOF'
Usage: scripts/eval_baseline.sh [options]

Runs baseline prompts against /api/chat and reports:
  - success rate
  - tool error rate
  - average response latency
  - per-case tool trace and stream diagnostics in JSON output

Options:
  --dataset <path>       Dataset path (default: tests/evals/dataset.yaml)
  --url <url>            Chat API URL (default: $ASSISTANT_API_URL or http://localhost:8081/api/chat)
  --out-dir <dir>        Output directory for run artifacts (default: tests/evals/results)
  --mock                 Run against local mock MCP config (tests/evals/config.mock.yaml)
  --mock-config <path>   Mock assistant config path (default: tests/evals/config.mock.yaml)
  --category <name>      Only run one category from dataset
  --limit <n>            Max number of cases to run (0 = all)
  --timeout <seconds>    Curl timeout per case (default: 120)
  --delay-ms <n>         Delay between cases in milliseconds (default: 150)
  --help                 Show this help

Auth:
  export ASSISTANT_COOKIE='grafana_session=...; other_cookie=...'
  export ASSISTANT_AUTH_HEADER='Authorization: Bearer ...'

Optional default dashboard context env vars:
  ASSISTANT_DASHBOARD_UID
  ASSISTANT_DASHBOARD_NAME   (default: Monitoring Assistant)
  ASSISTANT_TIME_FROM        (default: now-6h)
  ASSISTANT_TIME_TO          (default: now)

Dataset parsing:
  - JSON datasets are supported directly via jq.
  - YAML datasets with non-JSON syntax require yq for conversion.
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

epoch_ms() {
  # GNU date supports %3N; fall back to second precision on BSD/macOS date.
  local now
  now="$(date +%s%3N 2>/dev/null || true)"
  if [[ "$now" =~ ^[0-9]{13}$ ]]; then
    printf '%s\n' "$now"
    return
  fi
  printf '%s000\n' "$(date +%s)"
}

health_url_from_api() {
  local api_url="$1"
  if [[ "$api_url" == *"/api/chat" ]]; then
    printf '%s/healthz\n' "${api_url%/api/chat}"
    return
  fi
  printf '%s/healthz\n' "${api_url%/}"
}

cleanup() {
  if [[ "$MOCK_ASSISTANT_STARTED" == "true" ]] && [[ -n "$MOCK_ASSISTANT_PID" ]]; then
    kill "$MOCK_ASSISTANT_PID" 2>/dev/null || true
    wait "$MOCK_ASSISTANT_PID" 2>/dev/null || true
  fi
  if [[ -n "$tmp_dir" ]] && [[ -d "$tmp_dir" ]]; then
    rm -rf "$tmp_dir"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dataset)
      DATASET="$2"
      shift 2
      ;;
    --url)
      API_URL="$2"
      API_URL_EXPLICIT=true
      shift 2
      ;;
    --mock)
      MOCK_MODE=true
      shift
      ;;
    --mock-config)
      MOCK_CONFIG="$2"
      MOCK_MODE=true
      shift 2
      ;;
    --out-dir)
      OUT_DIR="$2"
      shift 2
      ;;
    --category)
      CATEGORY="$2"
      shift 2
      ;;
    --limit)
      LIMIT="$2"
      shift 2
      ;;
    --timeout)
      TIMEOUT_SECONDS="$2"
      shift 2
      ;;
    --delay-ms)
      DELAY_MS="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ "$MOCK_MODE" == "true" ]] && [[ "$API_URL_EXPLICIT" != "true" ]]; then
  API_URL="http://localhost:18081/api/chat"
fi

require_cmd curl
require_cmd jq
require_cmd awk

if [[ ! -f "$DATASET" ]]; then
  echo "Dataset not found: $DATASET" >&2
  exit 1
fi

if [[ "$LIMIT" != "0" ]] && ! [[ "$LIMIT" =~ ^[0-9]+$ ]]; then
  echo "--limit must be a non-negative integer" >&2
  exit 1
fi

if ! [[ "$TIMEOUT_SECONDS" =~ ^[0-9]+$ ]]; then
  echo "--timeout must be an integer number of seconds" >&2
  exit 1
fi

if ! [[ "$DELAY_MS" =~ ^[0-9]+$ ]]; then
  echo "--delay-ms must be a non-negative integer" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"
timestamp="$(date +%Y%m%d-%H%M%S)"
out_file="$OUT_DIR/baseline-$timestamp.json"

tmp_dir="$(mktemp -d)"
results_ndjson="$tmp_dir/results.ndjson"
touch "$results_ndjson"
trap cleanup EXIT

if [[ "$MOCK_MODE" == "true" ]]; then
  require_cmd make
  if [[ ! -f "$MOCK_CONFIG" ]]; then
    echo "Mock config not found: $MOCK_CONFIG" >&2
    exit 1
  fi

  # Build local binaries needed by mock mode without requiring Docker.
  if ! GOCACHE="${GOCACHE:-/tmp/go-build}" GOMODCACHE="${GOMODCACHE:-/tmp/go-mod-cache}" make build mock-mcp-build >/dev/null; then
    echo "Failed to build assistant/mock-mcp for --mock mode" >&2
    exit 1
  fi

  mock_health_url="$(health_url_from_api "$API_URL")"
  if ! curl -fsS "$mock_health_url" >/dev/null 2>&1; then
    ./bin/assistant -config "$MOCK_CONFIG" >"$tmp_dir/mock-assistant.log" 2>&1 &
    MOCK_ASSISTANT_PID="$!"
    MOCK_ASSISTANT_STARTED=true

    ready=false
    for _ in $(seq 1 90); do
      if curl -fsS "$mock_health_url" >/dev/null 2>&1; then
        ready=true
        break
      fi
      if ! kill -0 "$MOCK_ASSISTANT_PID" 2>/dev/null; then
        break
      fi
      sleep 1
    done

    if [[ "$ready" != "true" ]]; then
      echo "Mock assistant failed to become healthy at $mock_health_url" >&2
      if [[ -f "$tmp_dir/mock-assistant.log" ]]; then
        tail -n 120 "$tmp_dir/mock-assistant.log" >&2 || true
      fi
      exit 1
    fi
  fi
fi

dataset_json="$DATASET"
if ! jq -e '.cases and (.cases | type == "array")' "$dataset_json" >/dev/null 2>&1; then
  if command -v yq >/dev/null 2>&1; then
    dataset_json="$tmp_dir/dataset.json"
    if ! yq -o=json '.' "$DATASET" >"$dataset_json" 2>/dev/null; then
      if ! yq eval -o=json '.' "$DATASET" >"$dataset_json" 2>/dev/null; then
        echo "Failed to parse dataset with yq: $DATASET" >&2
        exit 1
      fi
    fi
  else
    echo "Dataset is not valid JSON. Install yq for YAML parsing support: $DATASET" >&2
    exit 1
  fi
fi

if ! jq -e '.cases and (.cases | type == "array")' "$dataset_json" >/dev/null 2>&1; then
  echo "Dataset must contain a top-level .cases array: $DATASET" >&2
  exit 1
fi

selection_filter='
  .cases
  | (if $category == "" then . else map(select(.category == $category)) end)
  | (if $limit > 0 then .[:$limit] else . end)
'

selected_count="$(jq -r --arg category "$CATEGORY" --argjson limit "$LIMIT" "$selection_filter | length" "$dataset_json")"
if [[ "$selected_count" -eq 0 ]]; then
  echo "No cases selected (category='$CATEGORY', limit=$LIMIT)." >&2
  exit 1
fi

mapfile -t selected_cases < <(jq -c --arg category "$CATEGORY" --argjson limit "$LIMIT" "$selection_filter | .[]" "$dataset_json")

delay_seconds="$(awk -v ms="$DELAY_MS" 'BEGIN { printf "%.3f", ms / 1000 }')"

echo "Running baseline eval"
if [[ "$MOCK_MODE" == "true" ]]; then
  echo "  mode:     mock"
  echo "  config:   $MOCK_CONFIG"
fi
echo "  dataset:  $DATASET"
echo "  api_url:  $API_URL"
echo "  cases:    $selected_count"
[[ -n "$CATEGORY" ]] && echo "  category: $CATEGORY"
echo "  timeout:  ${TIMEOUT_SECONDS}s"
echo "  out_file: $out_file"

idx=0
for case_json in "${selected_cases[@]}"; do
  idx=$((idx + 1))
  case_id="$(jq -r '.id' <<<"$case_json")"
  category="$(jq -r '.category' <<<"$case_json")"
  message="$(jq -r '.message' <<<"$case_json")"
  use_default_context="$(jq -r '.use_default_dashboard_context // false' <<<"$case_json")"

  dashboard_context="null"
  if jq -e '.dashboard_context != null' <<<"$case_json" >/dev/null 2>&1; then
    dashboard_context="$(jq -c '.dashboard_context' <<<"$case_json")"
  elif [[ "$use_default_context" == "true" ]]; then
    default_name="${ASSISTANT_DASHBOARD_NAME:-Monitoring Assistant}"
    default_from="${ASSISTANT_TIME_FROM:-now-6h}"
    default_to="${ASSISTANT_TIME_TO:-now}"
    if [[ -n "${ASSISTANT_DASHBOARD_UID:-}" ]]; then
      dashboard_context="$(jq -nc \
        --arg uid "$ASSISTANT_DASHBOARD_UID" \
        --arg name "$default_name" \
        --arg from "$default_from" \
        --arg to "$default_to" \
        '{uid:$uid,name:$name,time_range:{from:$from,to:$to}}')"
    else
      dashboard_context="$(jq -nc \
        --arg name "$default_name" \
        --arg from "$default_from" \
        --arg to "$default_to" \
        '{name:$name,time_range:{from:$from,to:$to}}')"
    fi
  fi

  payload="$(jq -nc --arg message "$message" '{message:$message}')"
  if [[ "$dashboard_context" != "null" ]]; then
    payload="$(jq -nc --arg message "$message" --argjson ctx "$dashboard_context" '{message:$message,dashboard_context:$ctx}')"
  fi

  sse_file="$tmp_dir/$idx.sse"
  chunks_file="$tmp_dir/$idx.chunks.jsonl"
  chunks_array_file="$tmp_dir/$idx.chunks.array.json"

  curl_args=(
    -sS
    -N
    -X POST "$API_URL"
    -H "Content-Type: application/json"
    -H "X-Requested-With: XMLHttpRequest"
    --max-time "$TIMEOUT_SECONDS"
    --data "$payload"
    -o "$sse_file"
    -w "%{http_code}"
  )
  curl_args+=(-H "X-Assistant-Eval-Case-ID: $case_id")
  if [[ -n "$COOKIE_HEADER" ]]; then
    curl_args+=(-H "Cookie: $COOKIE_HEADER")
  fi
  if [[ -n "$AUTH_HEADER" ]]; then
    curl_args+=(-H "$AUTH_HEADER")
  fi

  start_ms="$(epoch_ms)"
  http_status=""
  curl_error=""
  if ! http_status="$(curl "${curl_args[@]}")"; then
    curl_error="curl_request_failed"
  fi
  end_ms="$(epoch_ms)"
  latency_ms=$((end_ms - start_ms))

  success=false
  assistant_text=""
  error_message=""
  stream_errors="[]"
  tool_trace="[]"
  sse_event_count=0
  tool_invocations=0
  tool_results=0
  tool_errors=0
  session_id=""

  if [[ -n "$curl_error" ]]; then
    error_message="$curl_error"
  elif [[ "$http_status" != "200" ]]; then
    body="$(tr -d '\r' <"$sse_file" | head -c 300)"
    error_message="http_status_${http_status}"
    if [[ -n "$body" ]]; then
      error_message="$error_message: $body"
    fi
  else
    grep -a '^data: {' "$sse_file" | sed 's/^data: //' >"$chunks_file" || true

    if [[ ! -s "$chunks_file" ]]; then
      error_message="empty_sse_stream"
    elif ! jq -s '.' "$chunks_file" >"$chunks_array_file"; then
      error_message="invalid_sse_json"
    else
      sse_event_count="$(jq 'length' "$chunks_array_file")"
      session_id="$(jq -r 'map(select(.type == "start") | .session_id)[0] // ""' "$chunks_array_file")"
      has_done="$(jq -r 'any(.[]; .type == "done")' "$chunks_array_file")"
      stream_errors="$(jq -c '
        [
          .[]
          | select(.type == "error")
          | {
              message: (.message // ""),
              tool: (.tool // ""),
              tool_id: (.tool_id // "")
            }
        ]
      ' "$chunks_array_file")"
      error_message="$(jq -r 'map(select(.type == "error") | .message)[0] // ""' "$chunks_array_file")"
      assistant_text="$(jq -r '
        ([.[] | select(.type == "token" and (.message // "") != "") | .message] | join("")) as $tok
        | if ($tok | length) > 0
          then $tok
          else (map(select(.type == "complete") | .message)[0] // "")
          end
      ' "$chunks_array_file")"
      tool_trace="$(jq -c '
        def is_tool_error_result:
          if type == "object" then
            (
              (has("error") and (.error != null) and ((.error | tostring | length) > 0))
              and ((.error | type) != "boolean" or .error == true)
              or (
                has("errors")
                and (
                  ((.errors | type) == "array" and (.errors | length) > 0)
                  or ((.errors | type) == "string" and (.errors | length) > 0)
                )
              )
              or (has("ok") and (.ok == false))
              or (has("success") and (.success == false))
              or (((.status? // "" | tostring) | ascii_downcase) | test("^(error|failed|fail|failure|timeout|timed_out)$"))
              or (((.state? // "" | tostring) | ascii_downcase) | test("^(error|failed|fail|failure|timeout|timed_out)$"))
            )
          elif type == "string" then
            ((ascii_downcase | test("^(error:|failed:|timeout:)")))
          else
            false
          end;

        [
          .[]
          | select(.type == "tool")
          | if has("arguments") then
              {
                event: "call",
                tool: (.tool // ""),
                tool_id: (.tool_id // ""),
                reason: (.reason // ""),
                arguments: (.arguments // {})
              }
            elif has("result") then
              {
                event: "result",
                tool: (.tool // ""),
                tool_id: (.tool_id // ""),
                reason: (.reason // ""),
                result: .result,
                is_error: (.result | is_tool_error_result)
              }
            else
              empty
            end
        ]
      ' "$chunks_array_file")"
      tool_invocations="$(jq '[.[] | select(.event == "call")] | length' <<<"$tool_trace")"
      tool_results="$(jq '[.[] | select(.event == "result")] | length' <<<"$tool_trace")"
      tool_errors="$(jq '[.[] | select(.event == "result" and .is_error)] | length' <<<"$tool_trace")"

      if [[ "$has_done" == "true" ]] && [[ -z "$error_message" ]] && [[ -n "$assistant_text" ]]; then
        if [[ ! "$assistant_text" =~ ^[Ee]rror: ]]; then
          success=true
        else
          error_message="assistant_returned_error_prefix"
        fi
      else
        [[ -z "$error_message" ]] && error_message="missing_done_or_empty_response"
      fi
    fi
  fi

  expected_json="$(jq -c '.expected' <<<"$case_json")"
  case_result="$(
    jq -nc \
      --arg id "$case_id" \
      --arg category "$category" \
      --arg message "$message" \
      --argjson expected "$expected_json" \
      --argjson success "$success" \
      --argjson latency_ms "$latency_ms" \
      --argjson http_status "${http_status:-0}" \
      --arg session_id "$session_id" \
      --arg assistant_text "$assistant_text" \
      --arg error "$error_message" \
      --argjson stream_errors "$stream_errors" \
      --argjson sse_event_count "$sse_event_count" \
      --argjson tool_trace "$tool_trace" \
      --argjson tool_invocations "$tool_invocations" \
      --argjson tool_results "$tool_results" \
      --argjson tool_errors "$tool_errors" \
      '{
        id: $id,
        category: $category,
        message: $message,
        expected: $expected,
        success: $success,
        http_status: $http_status,
        latency_ms: $latency_ms,
        session_id: $session_id,
        assistant_text: $assistant_text,
        error: $error,
        stream_errors: $stream_errors,
        sse_event_count: $sse_event_count,
        tool_trace: $tool_trace,
        tool_invocations: $tool_invocations,
        tool_results: $tool_results,
        tool_errors: $tool_errors
      }'
  )"
  echo "$case_result" >>"$results_ndjson"

  if [[ "$success" == "true" ]]; then
    echo "[$idx/$selected_count] $case_id ($category): ok (${latency_ms}ms)"
  else
    echo "[$idx/$selected_count] $case_id ($category): fail (${latency_ms}ms) - $error_message"
  fi

  sleep "$delay_seconds"
done

jq -s \
  --arg generated_at "$(date -Iseconds)" \
  --arg dataset "$DATASET" \
  --arg api_url "$API_URL" \
  --argjson limit "$LIMIT" \
  --arg category "$CATEGORY" \
  '
  def pct($n; $d):
    if $d == 0 then 0 else (($n * 10000 / $d) | round / 100) end;

  . as $cases
  | ($cases | length) as $total
  | ($cases | map(select(.success)) | length) as $successes
  | ($cases | map(select(.success | not)) | length) as $failed
  | ($cases | map(.tool_results) | add // 0) as $tool_results
  | ($cases | map(.tool_errors) | add // 0) as $tool_errors
  | ($cases | map(.sse_event_count) | add // 0) as $sse_events
  | ($cases | [ .[].tool_trace[]? | select(.event == "call") | (.tool // "unknown") ]) as $tool_call_names
  | ($cases | [ .[].tool_trace[]? | select(.event == "result" and (.is_error // false)) | (.tool // "unknown") ]) as $tool_error_names
  | ($cases | map(.latency_ms) | add // 0) as $latency_sum
  | {
      generated_at: $generated_at,
      dataset: $dataset,
      api_url: $api_url,
      selection: {
        category: (if $category == "" then null else $category end),
        limit: $limit
      },
      totals: {
        cases: $total,
        success: $successes,
        failed: $failed,
        success_rate_pct: pct($successes; $total),
        total_sse_events: $sse_events,
        total_tool_results: $tool_results,
        total_tool_errors: $tool_errors,
        tool_error_rate_pct: pct($tool_errors; $tool_results),
        avg_latency_ms: (if $total == 0 then 0 else (($latency_sum / $total) | round) end)
      },
      tool_calls_by_name: (
        $tool_call_names
        | sort
        | group_by(.)
        | map({tool: .[0], count: length})
      ),
      tool_errors_by_name: (
        $tool_error_names
        | sort
        | group_by(.)
        | map({tool: .[0], count: length})
      ),
      failed_case_ids: ($cases | map(select(.success | not) | .id)),
      cases: $cases
    }
  ' "$results_ndjson" | jq '.' >"$out_file"

echo
echo "Baseline evaluation complete"
echo "  output:            $out_file"
echo "  success rate:      $(jq -r '.totals.success_rate_pct' "$out_file")%"
echo "  tool error rate:   $(jq -r '.totals.tool_error_rate_pct' "$out_file")%"
echo "  avg latency:       $(jq -r '.totals.avg_latency_ms' "$out_file") ms"
echo "  failed cases:      $(jq -r '.failed_case_ids | join(", ")' "$out_file")"
