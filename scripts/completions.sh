#!/usr/bin/env bash
set -euo pipefail

mkdir -p completions

echo "Building temporary camp-graph binary for completion generation..."
go build -o completions/.camp-graph-tmp ./cmd/camp-graph

echo "Generating completions..."
./completions/.camp-graph-tmp completion bash > completions/camp-graph.bash
./completions/.camp-graph-tmp completion zsh > completions/_camp-graph
./completions/.camp-graph-tmp completion fish > completions/camp-graph.fish

rm -f completions/.camp-graph-tmp
echo "Completions generated in completions/"
