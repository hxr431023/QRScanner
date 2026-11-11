package native

// lerf_test.go — 本文件为本项目自己写的代码，不是微软的代码。
// Checks the reusable LERF reader without touching any DLL.

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = "LERF.utf8\nQRScanner.V1.0.0\n\n0\n" +
	"# comment\n" +
	"<12>Saved to: {0}<12>\n" +
	"<13>Language:<13>\n" +
	"plain_key=plain_value\n"

func TestParse(t *testing.T) {
	table := Parse(sample)
	if table["12"] != "Saved to: {0}" {
		t.Fatalf("id 12 -> %q", table["12"])
	}
	if table["13"] != "Language:" {
		t.Fatalf("id 13 -> %q", table["13"])
	}
	if table["plain_key"] != "plain_value" {
		t.Fatalf("key=value -> %q", table["plain_key"])
	}
}

func TestGetf(t *testing.T) {
	table := Parse(sample)
	if got := table.Getf("12", `/tmp/a.png`); got != "Saved to: /tmp/a.png" {
		t.Fatalf("Getf -> %q", got)
	}
	// a missing id stays visible instead of silently blanking the UI
	if got := table.Get("999"); got != "999" {
		t.Fatalf("missing id -> %q", got)
	}
}

func TestLoadDirFallsBackToEnglish(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, "resources", "cmd")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "en.lerf"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	table, ok := LoadDir(dir, "zh", "en")
	if ok {
		t.Fatal("zh was missing, ok should be false")
	}
	if table.Get("13") != "Language:" {
		t.Fatal("fallback table was not used")
	}
}
