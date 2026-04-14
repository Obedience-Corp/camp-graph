#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
completions_dir="${repo_root}/completions"
tmp_binary="${completions_dir}/.camp-graph-tmp"
tmp_campaign="$(mktemp -d)"

cleanup() {
  rm -f "${tmp_binary}"
  rm -rf "${tmp_campaign}"
}

mkdir -p "${completions_dir}"
mkdir -p "${tmp_campaign}/.campaign"
trap cleanup EXIT

echo "Building temporary camp-graph binary for completion generation..."
(
  cd "${repo_root}"
  go build -o "${tmp_binary}" ./cmd/camp-graph
)

echo "Generating completions..."
(
  cd "${tmp_campaign}"
  CAMP_ROOT="${tmp_campaign}" "${tmp_binary}" completion bash > "${completions_dir}/camp-graph.bash"
  CAMP_ROOT="${tmp_campaign}" "${tmp_binary}" completion zsh > "${completions_dir}/_camp-graph"
  CAMP_ROOT="${tmp_campaign}" "${tmp_binary}" completion fish > "${completions_dir}/camp-graph.fish"
)

echo "Completions generated in ${completions_dir}/"
