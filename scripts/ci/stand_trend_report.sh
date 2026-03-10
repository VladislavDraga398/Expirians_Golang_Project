#!/usr/bin/env bash
set -euo pipefail

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Required tool not found: $1"
    exit 1
  }
}

need curl
need jq
need unzip

: "${GITHUB_TOKEN:?GITHUB_TOKEN is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"

WORKFLOW_FILE=${WORKFLOW_FILE:-ci.yml}
BRANCH=${BRANCH:-${GITHUB_REF_NAME:-main}}
LOOKBACK_RUNS=${LOOKBACK_RUNS:-10}
PER_PAGE=$((LOOKBACK_RUNS * 4))
if (( PER_PAGE < 20 )); then
  PER_PAGE=20
fi
if (( PER_PAGE > 100 )); then
  PER_PAGE=100
fi

OUTPUT_MD=${OUTPUT_MD:-/tmp/stand-trend-report.md}

api() {
  local url="$1"
  curl -fsSL \
    -H "Authorization: Bearer ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    "$url"
}

runs_url="https://api.github.com/repos/${GITHUB_REPOSITORY}/actions/workflows/${WORKFLOW_FILE}/runs?status=completed&branch=${BRANCH}&per_page=${PER_PAGE}"
runs_json="$(api "$runs_url")"

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

rows=()
collected=0

while IFS= read -r run; do
  run_id="$(jq -r '.id' <<<"$run")"
  run_number="$(jq -r '.run_number' <<<"$run")"
  run_date="$(jq -r '.created_at[0:10]' <<<"$run")"
  run_event="$(jq -r '.event' <<<"$run")"
  run_branch="$(jq -r '.head_branch // "-"' <<<"$run")"
  run_url="$(jq -r '.html_url' <<<"$run")"

  artifacts_json="$(api "https://api.github.com/repos/${GITHUB_REPOSITORY}/actions/runs/${run_id}/artifacts?per_page=100")"
  artifact_url="$(jq -r '
    .artifacts
    | map(select(.expired == false and (.name == "premerge-stand-artifacts" or .name == "nightly-stand-artifacts")))
    | first
    | .archive_download_url // empty
  ' <<<"$artifacts_json")"

  if [[ -z "$artifact_url" ]]; then
    continue
  fi

  run_dir="${tmp_dir}/run-${run_id}"
  mkdir -p "$run_dir"
  zip_path="${run_dir}/artifact.zip"
  api "$artifact_url" >"$zip_path"
  unzip -qq "$zip_path" -d "$run_dir"

  report_file="$(find "$run_dir" -type f \( -name 'load-gate-report*.json' -o -name 'nightly-load-gate-report*.json' -o -name 'load-gate-report.json' \) | head -n1)"
  if [[ -z "$report_file" ]]; then
    continue
  fi

  total="$(jq -r '.total_scenarios // 0' "$report_file")"
  error_rate="$(jq -r '.error_rate // 0' "$report_file")"
  avg_ms="$(jq -r '.scenario_latency_ms.avg // 0' "$report_file")"
  p95_ms="$(jq -r '.scenario_latency_ms.p95 // 0' "$report_file")"
  rps="$(jq -r '.rps // 0' "$report_file")"

  rows+=("| [#${run_number}](${run_url}) | ${run_date} | ${run_event} | ${run_branch} | ${total} | ${error_rate} | ${avg_ms} | ${p95_ms} | ${rps} |")
  collected=$((collected + 1))
  if (( collected >= LOOKBACK_RUNS )); then
    break
  fi
done < <(jq -c '.workflow_runs[] | select(.conclusion == "success")' <<<"$runs_json")

{
  echo "## Pre-Merge Stand Trend (${collected}/${LOOKBACK_RUNS})"
  echo
  if (( collected == 0 )); then
    echo "_No historical load reports found for branch \`${BRANCH}\`._"
  else
    echo "| Run | Date | Event | Branch | Total | Error Rate | Avg ms | P95 ms | RPS |"
    echo "|---|---|---|---|---:|---:|---:|---:|---:|"
    for row in "${rows[@]}"; do
      echo "$row"
    done
  fi
} >"$OUTPUT_MD"

cat "$OUTPUT_MD"
