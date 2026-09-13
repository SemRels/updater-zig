// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package plugin updates build.zig.zon files in-place.
package plugin

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// zonVersionPattern matches a top-level `.version = "..."` field in ZON
// (Zig Object Notation) syntax, as emitted by `zig fmt` and `zig init`.
var zonVersionPattern = regexp.MustCompile(`^(\s*\.version\s*=\s*)"[^"]*"(\s*,?\s*)$`)

// Updater updates the package version in build.zig.zon manifests.
type Updater struct{}

// NewUpdater creates an updater.
func NewUpdater() *Updater {
	return &Updater{}
}

// Update rewrites the top-level `.version` field of the manifest at path.
//
// build.zig.zon has no sections like Cargo.toml or package.json; instead the
// whole file is a single anonymous struct literal (`.{ ... }`) that may
// contain nested struct literals, most notably `.dependencies = .{ ... }`.
// Only the version field belonging to the outermost struct describes the
// package itself, so brace depth is tracked to avoid ever touching a field
// nested inside `.dependencies` (dependency entries do not carry a `.version`
// field today, but this keeps the updater forward-compatible and safe).
//
// The `.name` and `.fingerprint` fields are never touched: together they form
// the package's permanent identity and must survive version bumps unchanged.
func (u *Updater) Update(path, version string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	updated, err := updateContent(string(data), version)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func updateContent(content, version string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	lines := make([]string, 0)
	depth := 0
	updated := false

	for scanner.Scan() {
		line := scanner.Text()
		if depth == 1 && !updated && zonVersionPattern.MatchString(line) {
			line = zonVersionPattern.ReplaceAllString(line, `${1}"`+version+`"${2}`)
			updated = true
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan build.zig.zon: %w", err)
	}
	if !updated {
		return "", fmt.Errorf("package version not found in build.zig.zon")
	}
	return strings.Join(lines, "\n"), nil
}
