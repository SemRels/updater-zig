// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleZon = `.{
    .name = .demo,
    .version = "1.2.3",
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

func TestUpdaterUpdateBuildZigZon(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "build.zig.zon")
	if err := os.WriteFile(file, []byte(sampleZon), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := NewUpdater().Update(file, "1.3.0"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `.version = "1.3.0",`) {
		t.Fatalf("updated file = %s", got)
	}
	// name and fingerprint must remain untouched.
	if !strings.Contains(string(got), ".name = .demo,") {
		t.Fatalf("name was modified: %s", got)
	}
	if !strings.Contains(string(got), ".fingerprint = 0xdeadbeefcafef00d,") {
		t.Fatalf("fingerprint was modified: %s", got)
	}
}

func TestUpdaterMissingFile(t *testing.T) {
	t.Parallel()

	err := NewUpdater().Update(filepath.Join(t.TempDir(), "build.zig.zon"), "1.3.0")
	if err == nil || !strings.Contains(err.Error(), "read") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestUpdaterMissingVersion(t *testing.T) {
	t.Parallel()

	_, err := updateContent(".{\n    .name = .demo,\n}\n", "1.3.0")
	if err == nil || !strings.Contains(err.Error(), "package version not found") {
		t.Fatalf("expected version error, got %v", err)
	}
}

func TestUpdaterIgnoresNestedVersionFields(t *testing.T) {
	t.Parallel()

	// A hypothetical future dependency-level `.version` field must never be
	// confused with the top-level package version.
	content := `.{
    .name = .demo,
    .dependencies = .{
        .foo = .{
            .version = "9.9.9",
            .path = "../foo",
        },
    },
    .version = "1.0.0",
}
`
	updated, err := updateContent(content, "2.0.0")
	if err != nil {
		t.Fatalf("updateContent() error = %v", err)
	}
	if !strings.Contains(updated, `.version = "2.0.0",`) {
		t.Fatalf("top-level version not updated: %s", updated)
	}
	if !strings.Contains(updated, `.version = "9.9.9",`) {
		t.Fatalf("nested dependency version must remain untouched: %s", updated)
	}
}
