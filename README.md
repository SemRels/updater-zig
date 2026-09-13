# updater-zig

[![Latest Release](https://img.shields.io/github/v/release/SemRels/updater-zig?label=version&color=blue)](https://github.com/SemRels/updater-zig/releases/latest)

Updates the package version in `build.zig.zon`, the manifest file for [Zig](https://ziglang.org)'s build system and package manager.

This plugin is distributed as the standalone Go binary `semrel-plugin-updater-zig`. Semrel executes the binary as a subprocess, provides plugin configuration through `SEMREL_PLUGIN_*` environment variables, provides release context through `SEMREL_*` environment variables, reads standard output, and treats exit code `0` as success and any non-zero exit code as failure. Install the binary in `~/.semrel/plugins/` or anywhere on your `$PATH`.

## Versioning concept

`build.zig.zon` does not look like `Cargo.toml`, `package.json`, or `pubspec.yaml`. It is written in [ZON](https://ziglang.org/documentation/master/#Zig-Object-Notation) (Zig Object Notation) — the whole file is a single anonymous struct literal, and there are no `[section]` headers to scope a field to "the package" the way TOML does. Designing a safe, semver-aware updater for it means understanding four fields defined by the [official manifest documentation](https://github.com/ziglang/zig/blob/master/doc/build.zig.zon.md):

| Field | Required | Mutability on release | Purpose |
| --- | --- | --- | --- |
| `.name` | Yes | **Never changes** | Enum-literal key other packages use in their `dependencies` table. |
| `.fingerprint` | Auto-generated | **Never changes** | A 64-bit value (32-bit id + 32-bit checksum) that, together with `.name`, forms the package's permanent identity. Zig uses it to detect that one package is an *update* of another — regenerating it is only correct when hostile-forking an abandoned project. |
| `.version` | Yes | **Updated on every release** | A [semver](https://semver.org) string, max 32 bytes. This is the field semrel manages. |
| `.minimum_zig_version` | Optional | Managed separately | Advisory semver floor for the Zig toolchain; not currently enforced by the compiler and out of scope for this plugin. |

The versioning concept therefore has three rules, enforced by `internal/plugin/updater.go`:

1. **Only the top-level `.version` field is a release version.** Everything nested inside `.dependencies = .{ ... }` describes *other* packages this one depends on (each with its own `url`/`hash` or `path`), not this package. The updater tracks brace depth (`{` / `}`) while scanning the file line by line and only rewrites a `.version = "..."` match at depth 1, i.e. a direct child of the root struct. This keeps the plugin forward-compatible even if a future Zig release adds per-dependency version metadata.
2. **Identity fields are immutable during a release.** `.name` and `.fingerprint` are what let Zig's package manager recognize "this is a newer version of the same package" instead of "this is an unrelated package with a similar name." The updater never touches them — it only ever rewrites the quoted string after `.version =`.
3. **The `v` tag prefix is stripped, the semver value is not reformatted.** semrel resolves tags like `v1.4.0`; the plugin trims a leading `v` before writing, since `build.zig.zon` expects a bare [semver](https://semver.org) string (`"1.4.0"`), consistent with how Zig's own tooling (`zig build --fetch`, `zig init`) reads the field.

This mirrors the same pattern already used by sibling plugins in this monorepo (`updater-cargo` for `Cargo.toml`, `updater-pubspec` for `pubspec.yaml`, etc.): find the one authoritative version field for *this* package, leave every other field — including nested dependency metadata — untouched, and fail loudly if no version field is found rather than silently doing nothing.

### Example

Given:

```zon
.{
    .name = .my_lib,
    .version = "1.2.3",
    .fingerprint = 0xdeadbeefcafef00d,
    .minimum_zig_version = "0.13.0",
    .dependencies = .{
        .foo = .{
            .url = "https://example.com/foo-1.0.0.tar.gz",
            .hash = "1220abcdef...",
        },
    },
    .paths = .{
        "build.zig",
        "build.zig.zon",
        "src",
    },
}
```

Running the plugin with `SEMREL_VERSION=v1.3.0` rewrites only the top-level version:

```zon
.{
    .name = .my_lib,
    .version = "1.3.0",
    .fingerprint = 0xdeadbeefcafef00d,
    ...
```

`.name`, `.fingerprint`, and everything under `.dependencies` are byte-for-byte identical.

## Installation

### Binary

```bash
go install github.com/SemRels/updater-zig/cmd/plugin@latest
```

### Docker

Pre-built, multi-platform images (linux/amd64, linux/arm64) are published to the GitHub Container Registry on every release:

```bash
docker pull ghcr.io/semrels/updater-zig:latest
```

Images are signed with [cosign](https://github.com/sigstore/cosign) and include a full SBOM attestation. Verify the signature:

```bash
cosign verify ghcr.io/semrels/updater-zig:latest \
  --certificate-identity-regexp 'https://github.com/SemRels/updater-zig/.github/workflows/release.yml.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Configuration

```yaml
plugins:
  - name: updater-zig
    path: ~/.semrel/plugins/semrel-plugin-updater-zig
    env:
      SEMREL_PLUGIN_FILE: "build.zig.zon"
```

## `SEMREL_PLUGIN_*` variables

| Name | Required | Description | Default |
| --- | --- | --- | --- |
| `SEMREL_PLUGIN_FILE` | Optional | Path to the `build.zig.zon` manifest to update. | build.zig.zon |

## `SEMREL_*` release context used

| Variable | Description |
| --- | --- |
| `SEMREL_VERSION` | Resolved release version for the current run. |
| `SEMREL_NEXT_VERSION` | Next version computed by semrel for the release. |
| `SEMREL_DRY_RUN` | Whether semrel is running in dry-run mode. |

## Example behavior

The plugin rewrites the top-level package version in the selected manifest file to the next release version, leaving `.name`, `.fingerprint`, `.minimum_zig_version`, and `.dependencies` untouched. In dry-run mode it reports the change without writing the file.
