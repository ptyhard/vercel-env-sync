package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// parseFlags のテスト

func TestParseFlags_Defaults(t *testing.T) {
	opts := parseFlags([]string{})
	if opts.env != ".env" {
		t.Errorf("デフォルト env: got %q, want .env", opts.env)
	}
	if opts.def != "vercel-env.yaml" {
		t.Errorf("デフォルト def: got %q, want vercel-env.yaml", opts.def)
	}
	if opts.dryRun {
		t.Error("デフォルト dryRun: got true, want false")
	}
	if opts.yes {
		t.Error("デフォルト yes: got true, want false")
	}
}

func TestParseFlags_EnvFlag(t *testing.T) {
	opts := parseFlags([]string{"--env", ".env.production"})
	if opts.env != ".env.production" {
		t.Errorf("--env: got %q, want .env.production", opts.env)
	}
}

func TestParseFlags_EnvFlagEquals(t *testing.T) {
	opts := parseFlags([]string{"--env=.env.staging"})
	if opts.env != ".env.staging" {
		t.Errorf("--env=: got %q, want .env.staging", opts.env)
	}
}

func TestParseFlags_DefFlag(t *testing.T) {
	opts := parseFlags([]string{"--def", "custom.yaml"})
	if opts.def != "custom.yaml" {
		t.Errorf("--def: got %q, want custom.yaml", opts.def)
	}
}

func TestParseFlags_DryRun(t *testing.T) {
	opts := parseFlags([]string{"--dry-run"})
	if !opts.dryRun {
		t.Error("--dry-run: got false, want true")
	}
}

func TestParseFlags_Yes(t *testing.T) {
	for _, arg := range []string{"--yes", "-yes", "-y"} {
		opts := parseFlags([]string{arg})
		if !opts.yes {
			t.Errorf("%s: got false, want true", arg)
		}
	}
}

// parseDotenv のテスト

func TestParseDotenv_Basic(t *testing.T) {
	input := "FOO=bar\nBAZ=qux\n"
	got := parseDotenv(input)
	if got["FOO"] != "bar" {
		t.Errorf("FOO: got %q, want bar", got["FOO"])
	}
	if got["BAZ"] != "qux" {
		t.Errorf("BAZ: got %q, want qux", got["BAZ"])
	}
}

func TestParseDotenv_SkipsComments(t *testing.T) {
	input := "# comment\nFOO=bar\n"
	got := parseDotenv(input)
	if _, ok := got["# comment"]; ok {
		t.Error("コメント行がキーとして解釈された")
	}
	if got["FOO"] != "bar" {
		t.Errorf("FOO: got %q, want bar", got["FOO"])
	}
}

func TestParseDotenv_QuotedValues(t *testing.T) {
	input := `FOO="hello world"` + "\nBAR='single'\n"
	got := parseDotenv(input)
	if got["FOO"] != "hello world" {
		t.Errorf("ダブルクォート: got %q, want \"hello world\"", got["FOO"])
	}
	if got["BAR"] != "single" {
		t.Errorf("シングルクォート: got %q, want single", got["BAR"])
	}
}

func TestParseDotenv_ExportPrefix(t *testing.T) {
	input := "export FOO=bar\n"
	got := parseDotenv(input)
	if got["FOO"] != "bar" {
		t.Errorf("export 付き: got %q, want bar", got["FOO"])
	}
}

func TestParseDotenv_EmptyLines(t *testing.T) {
	input := "\n\nFOO=bar\n\n"
	got := parseDotenv(input)
	if len(got) != 1 {
		t.Errorf("空行を含むとき: got %d keys, want 1", len(got))
	}
}

// normalizeTarget のテスト

func TestNormalizeTarget_Nil(t *testing.T) {
	got := normalizeTarget(nil)
	if got != nil {
		t.Errorf("nil: got %v, want nil", got)
	}
}

func TestNormalizeTarget_String(t *testing.T) {
	got := normalizeTarget("production")
	if len(got) != 1 || got[0] != "production" {
		t.Errorf("文字列: got %v, want [production]", got)
	}
}

func TestNormalizeTarget_Slice(t *testing.T) {
	got := normalizeTarget([]interface{}{"production", "preview"})
	if len(got) != 2 || got[0] != "production" || got[1] != "preview" {
		t.Errorf("スライス: got %v, want [production preview]", got)
	}
}

// --version フラグの統合テスト（バイナリをビルドして実行）

func TestVersionFlag(t *testing.T) {
	bin := t.TempDir() + "/vercel-env-sync-test"
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ビルド失敗: %s\n%s", err, out)
	}

	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		t.Fatalf("--version 実行失敗: %s", err)
	}
	got := strings.TrimSpace(string(out))
	if !strings.HasPrefix(got, "vercel-env-sync version ") {
		t.Errorf("--version 出力: got %q, want prefix \"vercel-env-sync version \"", got)
	}
}

func TestVersionFlag_ExitsZero(t *testing.T) {
	bin := t.TempDir() + "/vercel-env-sync-test"
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("ビルド失敗: %s\n%s", err, out)
	}
	cmd := exec.Command(bin, "--version")
	if err := cmd.Run(); err != nil {
		t.Errorf("--version は exit 0 であるべき: %s", err)
	}
}

func TestHelpFlag_ExitsZero(t *testing.T) {
	bin := t.TempDir() + "/vercel-env-sync-test"
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("ビルド失敗: %s\n%s", err, out)
	}
	cmd := exec.Command(bin, "--help")
	if err := cmd.Run(); err != nil {
		t.Errorf("--help は exit 0 であるべき: %s", err)
	}
}

func TestDryRunFlag_NoTokenRequired(t *testing.T) {
	bin := t.TempDir() + "/vercel-env-sync-test"
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("ビルド失敗: %s\n%s", err, out)
	}

	dir := t.TempDir()
	envFile := dir + "/.env"
	defFile := dir + "/vercel-env.yaml"
	if err := os.WriteFile(envFile, []byte("FOO=bar\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defFile, []byte("variables:\n  FOO: {type: plain}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "--dry-run", "--env", envFile, "--def", defFile)
	cmd.Env = append(os.Environ(), "VERCEL_PROJECT_ID=dummy-project")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("--dry-run は VERCEL_TOKEN なしで成功するべき: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "[dry-run]") {
		t.Errorf("dry-run 出力に [dry-run] が含まれない: %s", out)
	}
}
