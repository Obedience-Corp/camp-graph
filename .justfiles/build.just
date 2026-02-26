# Build recipes
# SYNC: must match root justfile
binary_name := "camp-graph"
bin_dir := "bin"
main_path := "./cmd/camp-graph"

# Show available build commands
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# Quick development build (no optimization)
[no-cd]
dev:
    @mkdir -p {{bin_dir}}
    go build -o {{bin_dir}}/{{binary_name}} {{main_path}}

# Build for Linux AMD64
[no-cd]
linux:
    GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-linux-amd64 {{main_path}}

# Build for macOS Intel
[no-cd]
darwin:
    GOOS=darwin GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-amd64 {{main_path}}

# Build for macOS Apple Silicon
[no-cd]
darwin-arm64:
    GOOS=darwin GOARCH=arm64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-arm64 {{main_path}}

# Build for Windows AMD64
[no-cd]
windows:
    GOOS=windows GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-windows-amd64.exe {{main_path}}

# Build for all platforms
all: linux darwin darwin-arm64 windows
    @echo "All platform builds complete"
