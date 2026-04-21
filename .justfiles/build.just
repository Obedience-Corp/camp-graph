# Build recipes
# SYNC: must match root justfile
project_root := justfile_directory()
binary_name := "camp-graph"
bin_dir := "bin"
main_path := "./cmd/camp-graph"

# Show available build commands
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# Quick development build (no optimization)
dev:
    @cd "{{project_root}}" && mkdir -p {{bin_dir}} && go build -o {{bin_dir}}/{{binary_name}} {{main_path}}

# Build for Linux AMD64
linux:
    cd "{{project_root}}" && GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-linux-amd64 {{main_path}}

# Build for macOS Intel
darwin:
    cd "{{project_root}}" && GOOS=darwin GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-amd64 {{main_path}}

# Build for macOS Apple Silicon
darwin-arm64:
    cd "{{project_root}}" && GOOS=darwin GOARCH=arm64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-arm64 {{main_path}}

# Build for Windows AMD64
windows:
    cd "{{project_root}}" && GOOS=windows GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-windows-amd64.exe {{main_path}}

# Build for all platforms
all: linux darwin darwin-arm64 windows
    @echo "All platform builds complete"
