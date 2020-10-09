package main

// ============================================================================
// GO
// ============================================================================
// Gen and Build main
// go:generate /usr/bin/env bash -c "echo 'Building example golang binaries (CLI and Server)'"
// go:generate /usr/bin/env bash -c "mkdir -p bin-all/{server,cli,client}"
// go:generate go build -v -o bin-all/server ./example/server/main.go
// go:generate go build -v -o bin-all/cli ./example/cli/main.go
