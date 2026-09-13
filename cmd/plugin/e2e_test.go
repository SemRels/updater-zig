// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

//go:build e2e

// Package main_test contains an end-to-end test that builds the real
// semrel-plugin-updater-zig binary and executes it exactly the way semrel
// core does: as a subprocess, configured through SEMREL_* and
// SEMREL_PLUGIN_* environment variables, reading its exit code and stdout.
//
// Run with: go test -tags e2e ./cmd/plugin/...
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildPluginBinary compiles the plugin exactly as the release workflow
// does and returns the path to the resulting binary.
func buildPluginBinary(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	name := "semrel-plugin-updater-zig"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binPath := filepath.Join(dir, name)

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return binPath
}

const fixtureZon = `.{
    .name = .demo,
    .version = "0.1.0",
    .fingerprint = 0xdeadbeefcafef00d,
    .minimum_zig_version = "0.13.0",
    .dependencies = .{
        .foo = .{
            .url = "https://example.com/foo-1.0.0.tar.gz",
            .hash = "1220abcdef",
        },
    },
    .paths = .{
        "build.zig",
        "build.zig.zon",
        "src",
    },
}
`

// TestE2E_UpdatesRealBuildZigZon runs the compiled plugin binary as an
// actual subprocess against a realistic build.zig.zon fixture and verifies
// the file on disk end-to-end, mirroring exactly how semrel core invokes
// updater plugins in production.
func TestE2E_UpdatesRealBuildZigZon(t *testing.T) {
	binPath := buildPluginBinary(t)

	dir := t.TempDir()
	zonPath := filepath.Join(dir, "build.zig.zon")
	if err := os.WriteFile(zonPath, []byte(fixtureZon), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		"SEMREL_VERSION=v0.2.0",
		"SEMREL_PLUGIN_FILE="+zonPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin binary failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "updated "+zonPath+" to version 0.2.0") {
		t.Fatalf("unexpected output: %s", out)
	}

	got, err := os.ReadFile(zonPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `.version = "0.2.0",`) {
		t.Fatalf("build.zig.zon was not updated on disk:\n%s", got)
	}
	if !strings.Contains(string(got), ".name = .demo,") {
		t.Fatalf("identity field .name was modified:\n%s", got)
	}
	if !strings.Contains(string(got), ".fingerprint = 0xdeadbeefcafef00d,") {
		t.Fatalf("identity field .fingerprint was modified:\n%s", got)
	}
	if !strings.Contains(string(got), `.hash = "1220abcdef",`) {
		t.Fatalf("nested dependency field was modified:\n%s", got)
	}
}

// TestE2E_DryRunLeavesFileUntouched exercises SEMREL_DRY_RUN end-to-end:
// the binary must report the change without ever writing to disk.
func TestE2E_DryRunLeavesFileUntouched(t *testing.T) {
	binPath := buildPluginBinary(t)

	dir := t.TempDir()
	zonPath := filepath.Join(dir, "build.zig.zon")
	if err := os.WriteFile(zonPath, []byte(fixtureZon), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		"SEMREL_VERSION=v0.2.0",
		"SEMREL_PLUGIN_FILE="+zonPath,
		"SEMREL_DRY_RUN=true",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin binary failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "[dry-run]") {
		t.Fatalf("unexpected output: %s", out)
	}

	got, err := os.ReadFile(zonPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `.version = "0.1.0",`) {
		t.Fatalf("dry-run must not modify the file on disk:\n%s", got)
	}
}

// TestE2E_MissingVersionFails verifies the subprocess exit code contract:
// semrel core treats any non-zero exit code as plugin failure.
func TestE2E_MissingVersionFails(t *testing.T) {
	binPath := buildPluginBinary(t)

	cmd := exec.Command(binPath)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit code, got success; output:\n%s", out)
	}
	if !strings.Contains(string(out), "SEMREL_VERSION is required") {
		t.Fatalf("unexpected output: %s", out)
	}
}
