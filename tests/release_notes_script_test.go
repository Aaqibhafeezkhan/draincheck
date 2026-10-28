//go:build !windows

package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const changelogFixture = `# Changelog

## [Unreleased]

### Added

- Not released yet.

## [0.2.0] - 2026-12-01

### Fixed

- Second.

## [0.1.0] - 2026-11-01

### Added

- First.

#### Analyzer rules

- ` + "`RULE`" + `: text.

## [0.0.1] - 2026-10-16

- Old.

[Unreleased]: https://example.invalid/compare/v0.2.0...develop
[0.2.0]: https://example.invalid/releases/tag/v0.2.0
[0.1.0]: https://example.invalid/releases/tag/v0.1.0
`

// runReleaseNotes runs .github/scripts/release-notes.sh against a changelog.
func runReleaseNotes(t *testing.T, changelog, tag string) (string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(changelog), 0o600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join("..", ".github", "scripts", "release-notes.sh")
	out, err := exec.Command("bash", script, tag, path).CombinedOutput()
	return string(out), err
}

func TestReleaseNotesExtractsOnlyTheTaggedSection(t *testing.T) {
	out, err := runReleaseNotes(t, changelogFixture, "v0.1.0")
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	want := "### Added\n\n- First.\n\n#### Analyzer rules\n\n- `RULE`: text.\n"
	if out != want {
		t.Errorf("notes:\n%q\nwant:\n%q", out, want)
	}
}

func TestReleaseNotesLastSectionStopsAtLinkReferences(t *testing.T) {
	cl := changelogFixture + "[0.0.1]: https://example.invalid/releases/tag/v0.0.1\n"
	out, err := runReleaseNotes(t, cl, "v0.0.1")
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	if out != "- Old.\n" {
		t.Errorf("notes: %q, want %q", out, "- Old.\n")
	}
}

func TestReleaseNotesFailsWithoutASection(t *testing.T) {
	for _, tag := range []string{"v9.9.9", "0.1.0", "v0.1.0-rc.1", ""} {
		if out, err := runReleaseNotes(t, changelogFixture, tag); err == nil {
			t.Errorf("tag %q: want failure, got:\n%s", tag, out)
		}
	}
}

func TestReleaseNotesFailsOnEmptySection(t *testing.T) {
	cl := "## [Unreleased]\n\n## [0.3.0] - 2026-12-02\n\n\n## [0.2.0] - 2026-12-01\n\n- x\n"
	if out, err := runReleaseNotes(t, cl, "v0.3.0"); err == nil {
		t.Errorf("want failure for an empty section, got:\n%s", out)
	}
}
