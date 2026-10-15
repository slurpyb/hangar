package discover

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPiPackageDeclarations(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"package.json": `{"pi":{
			"skills":["./resources/skills", "!disabled"],
			"prompts":["./resources/{text,other}/**/*.md", "!wip.md", "!restore.md", "+resources/text/restore.md", "+resources/text/off.md", "-resources/text/off.md"],
			"subagents":{"agents":["./definitions", "./definitions/review.md", "!./definitions/wip.md", "./excluded", "!./excluded"]}
		}}`,
		"resources/skills/check/SKILL.md":    "---\nname: check\n---\nCheck code",
		"resources/skills/disabled/SKILL.md": "Disabled skill",
		"resources/text/check.md":            "Review $ARGUMENTS",
		"resources/text/deep/build.md":       "Build",
		"resources/text/wip.md":              "Excluded",
		"resources/text/restore.md":          "Restored",
		"resources/text/off.md":              "Excluded despite force-include",
		"resources/text/.hidden.md":          "Hidden",
		"definitions/review.md":              "---\nname: check\ndescription: Check changes\n---\nReview code",
		"definitions/wip.md":                 "Excluded agent",
		"definitions/nested/hidden.md":       "Nonrecursive agent scan",
		"excluded/hidden.md":                 "Excluded directory",
		"agents/undeclared.md":               "Undeclared agent",
		"prompts/undeclared.md":              "Undeclared prompt",
		"skills/undeclared/SKILL.md":         "Undeclared skill",
		"rules/general.md":                   "Rules remain discoverable independently of Pi manifests",
	}
	for name, body := range files {
		write(t, filepath.Join(root, name), body)
	}
	got, err := DiscoverContent(root)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, sk := range got {
		keys = append(keys, sk.Key())
	}
	want := []string{"agent:check", "prompt:build", "prompt:check", "prompt:restore", "rule:rules", "skill:check"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("got %v, want %v", keys, want)
	}
	if got[0].RelPath != "definitions/review.md" || got[0].Description != "Check changes" {
		t.Fatalf("agent metadata: %#v", got[0])
	}
}

func TestPiPackageEmptyManifest(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "package.json"), `{"pi":{"skills":[],"extensions":["./src/index.ts"]}}`)
	write(t, filepath.Join(root, "skills/check/SKILL.md"), "Undeclared")
	write(t, filepath.Join(root, "prompts/check.md"), "Undeclared")
	write(t, filepath.Join(root, "agents/check.md"), "Undeclared")
	got, err := DiscoverContent(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("extension application was treated as content: %v, %v", got, err)
	}
}

func TestPiPackageAgentDeclarations(t *testing.T) {
	for _, manifest := range []string{
		`{"pi":{"subagents":["./definitions"]}}`,
		`{"pi-subagents":{"agents":["./definitions"]}}`,
		`{"pi-subagents":["./definitions"]}`,
		`{"pi":{"subagents":{"agents":["./definitions"]}},"pi-subagents":["./missing"]}`,
	} {
		root := t.TempDir()
		write(t, filepath.Join(root, "package.json"), manifest)
		write(t, filepath.Join(root, "definitions/check.md"), "Check")
		got, err := DiscoverContent(root)
		if err != nil || len(got) != 1 || got[0].Key() != "agent:check" {
			t.Fatalf("%s: %v, %v", manifest, got, err)
		}
	}
}

func TestPiPackageInvalidResources(t *testing.T) {
	for _, test := range []struct{ manifest, message string }{
		{`{"pi":{"subagents":["./definitions/*.md"]}}`, "not globs"},
		{`{"pi":{"prompts":["../outside.md"]}}`, "escapes root"},
		{`{"pi":{"prompts":["./missing.md"]}}`, "missing.md"},
	} {
		root := t.TempDir()
		write(t, filepath.Join(root, "package.json"), test.manifest)
		_, err := DiscoverContent(root)
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Fatalf("got %v, want %q", err, test.message)
		}
	}
}
