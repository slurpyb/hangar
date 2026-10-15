package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileRecursiveRepeatAndOwnership(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "rules")
	inputs := map[string]string{
		"00-general.md":                "# General\n<!-- preserve this -->\n",
		"backend/10-api.md":            "# API\n",
		"backend/rules/20-database.md": "# Database\n",
		"frontend/30-ui.md":            "# UI",
		"notes.txt":                    "ignored",
	}
	for name, data := range inputs {
		p := filepath.Join(src, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	want := "# General\n<!-- preserve this -->\n\n# API\n\n# Database\n\n# UI"
	for _, name := range []string{"APPEND_SYSTEM.md", "AGENTS.md", "AGENTS.append.md"} {
		dst := filepath.Join(base, name)
		for i := 0; i < 2; i++ {
			if err := Compile(src, dst); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != want {
				t.Fatalf("%s: got %q", name, got)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(src, "backend/rules/20-database.md"), []byte("Changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Compile(src, filepath.Join(base, "APPEND_SYSTEM.md")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(base, "APPEND_SYSTEM.md"))
	if strings.Count(string(data), "Changed") != 1 || strings.Contains(string(data), "Database") {
		t.Fatalf("stale output: %s", data)
	}
	if _, err := os.Stat(filepath.Join(src, "backend/APPEND_SYSTEM.md")); !os.IsNotExist(err) {
		t.Fatal("nested output created")
	}
	if err := Compile(src, filepath.Join(src, "AGENTS.md")); err == nil {
		t.Fatal("accepted output inside source")
	}
	unmanaged := filepath.Join(base, "other", "AGENTS.md")
	os.MkdirAll(filepath.Dir(unmanaged), 0755)
	os.WriteFile(unmanaged, []byte("Keep me"), 0644)
	if err := Compile(src, unmanaged); err == nil || !strings.Contains(err.Error(), "unmanaged") {
		t.Fatalf("expected ownership error, got %v", err)
	}
}

func TestCompileFailure(t *testing.T) {
	base := t.TempDir()
	if err := Compile(filepath.Join(base, "missing"), filepath.Join(base, "AGENTS.md")); err == nil {
		t.Fatal("accepted missing source")
	}
	empty := filepath.Join(base, "empty")
	os.Mkdir(empty, 0755)
	if err := Compile(empty, filepath.Join(base, "AGENTS.md")); err == nil || !strings.Contains(err.Error(), "no .md") {
		t.Fatalf("got %v", err)
	}
}
