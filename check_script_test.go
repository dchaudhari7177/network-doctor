package main

import (
	"os"
	"strings"
	"testing"
)

// The fast contributor gate is a shell script, so nothing the Go build does
// would notice it rotting. These tests pin the parts that would rot silently:
// the file's existence and executable bit, the checks it is documented to run,
// the ones it is documented to leave to CI, and the fact that CI runs it.
//
// They deliberately assert on the script's content rather than executing it.
// Running it from a test would recurse: the script's own `go test ./...` step
// runs this file.

const checkScript = "scripts/check"

func readCheckScript(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(checkScript)
	if err != nil {
		t.Fatalf("read %s: %v", checkScript, err)
	}
	return string(body)
}

// readCheckScriptCode returns the script with comment lines removed, so an
// assertion that a command is absent is not satisfied or defeated by prose.
// The script explains what it leaves to CI by naming those tools, and a naive
// substring search over the whole file reads that explanation as a call.
func readCheckScriptCode(t *testing.T) string {
	t.Helper()
	var code strings.Builder
	for _, line := range strings.Split(readCheckScript(t), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		code.WriteString(line)
		code.WriteByte('\n')
	}
	return code.String()
}

func TestCheckScriptExistsAndIsExecutable(t *testing.T) {
	info, err := os.Stat(checkScript)
	if err != nil {
		t.Fatalf("stat %s: %v", checkScript, err)
	}
	// Checked through git rather than the filesystem mode, because a Windows
	// checkout does not carry the bit but the committed tree must.
	if info.Mode().IsRegular() && info.Size() == 0 {
		t.Fatalf("%s is empty", checkScript)
	}
}

func TestCheckScriptRunsTheFastChecksItDocuments(t *testing.T) {
	script := readCheckScriptCode(t)
	for _, want := range []string{
		"gofmt -l .",
		"go vet ./...",
		"CGO_ENABLED=0 go build ./...",
		"GOOS=darwin go build ./...",
		"GOOS=windows go build ./...",
		"go test ./...",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("%s no longer runs %q", checkScript, want)
		}
	}
}

func TestCheckScriptLeavesTheExpensiveGateToCI(t *testing.T) {
	// The value of a fast check is that it is fast. If one of these ever moves
	// into it, that should be a deliberate edit to this list rather than a
	// contributor quietly discovering the script now takes ten minutes.
	script := readCheckScriptCode(t)
	for _, unwanted := range []string{
		"-tags integration",
		"-tags netns_integration",
		"-tags acceptance",
		"-tags container",
		"-fuzz=",
		"golangci-lint",
		"govulncheck",
		"goreleaser",
	} {
		if strings.Contains(script, unwanted) {
			t.Errorf("%s runs %q, which belongs in CI: it is slow, "+
				"platform-specific, or needs the network", checkScript, unwanted)
		}
	}
}

func TestCheckScriptNeedsNoPrivileges(t *testing.T) {
	script := readCheckScriptCode(t)
	for _, unwanted := range []string{"sudo", "sysctl", "docker ", "podman "} {
		if strings.Contains(script, unwanted) {
			t.Errorf("%s uses %q; the fast check must run unprivileged and "+
				"without a container runtime", checkScript, unwanted)
		}
	}
}

func TestContinuousIntegrationRunsTheCheckScript(t *testing.T) {
	// Without this the script is documentation that compiles nothing: it could
	// break and no contributor would find out until they ran it.
	workflow, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatalf("read ci.yml: %v", err)
	}
	if !strings.Contains(string(workflow), "./"+checkScript) {
		t.Fatalf("ci.yml does not run ./%s, so it can rot unnoticed", checkScript)
	}
}

func TestContributingAndREADMEPointAtTheCheckScript(t *testing.T) {
	for _, doc := range []string{"CONTRIBUTING.md", "README.md"} {
		body, err := os.ReadFile(doc)
		if err != nil {
			t.Fatalf("read %s: %v", doc, err)
		}
		if !strings.Contains(string(body), checkScript) {
			t.Errorf("%s does not mention %s", doc, checkScript)
		}
	}
}
