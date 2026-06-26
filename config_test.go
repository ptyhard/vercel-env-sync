package main

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// parseFlags のテスト

func TestParseFlags_Defaults(t *testing.T) {
	opts := parseFlags([]string{})
	if opts.env != ".env" {
		t.Errorf("デフォルト env: got %q, want .env", opts.env)
	}
	if opts.def != "env-sync.yaml" {
		t.Errorf("デフォルト def: got %q, want env-sync.yaml", opts.def)
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

func TestParseFlags_DefaultProvider(t *testing.T) {
	opts := parseFlags([]string{})
	if opts.provider != "vercel" {
		t.Errorf("provider のデフォルト = %q, want %q", opts.provider, "vercel")
	}
}

func TestParseFlags_ProviderVercel(t *testing.T) {
	opts := parseFlags([]string{"--provider", "vercel"})
	if opts.provider != "vercel" {
		t.Errorf("provider = %q, want vercel", opts.provider)
	}
}

func TestParseFlags_ProviderGitHub(t *testing.T) {
	opts := parseFlags([]string{"--provider", "github"})
	if opts.provider != "github" {
		t.Errorf("provider = %q, want github", opts.provider)
	}
}

func TestParseFlags_ProviderEqualForm(t *testing.T) {
	opts := parseFlags([]string{"--provider=github"})
	if opts.provider != "github" {
		t.Errorf("--provider=github が解析されない: got %q", opts.provider)
	}
}

// ProviderVal の YAML パーステスト

func TestProviderVal_UnmarshalYAML_String(t *testing.T) {
	type T struct {
		Provider *ProviderVal `yaml:"provider"`
	}
	var v T
	if err := yaml.Unmarshal([]byte("provider: vercel"), &v); err != nil {
		t.Fatalf("Unmarshal エラー: %v", err)
	}
	if v.Provider == nil || len(v.Provider.Values) != 1 || v.Provider.Values[0] != "vercel" {
		t.Errorf("ProviderVal = %v, want [vercel]", v.Provider)
	}
}

func TestProviderVal_UnmarshalYAML_Sequence(t *testing.T) {
	type T struct {
		Provider *ProviderVal `yaml:"provider"`
	}
	var v T
	if err := yaml.Unmarshal([]byte("provider: [vercel, github]"), &v); err != nil {
		t.Fatalf("Unmarshal エラー: %v", err)
	}
	if v.Provider == nil || len(v.Provider.Values) != 2 {
		t.Fatalf("ProviderVal.Values len = %d, want 2", len(v.Provider.Values))
	}
	if v.Provider.Values[0] != "vercel" || v.Provider.Values[1] != "github" {
		t.Errorf("ProviderVal.Values = %v, want [vercel github]", v.Provider.Values)
	}
}

func TestProviderVal_UnmarshalYAML_Nil(t *testing.T) {
	type T struct {
		Provider *ProviderVal `yaml:"provider"`
	}
	var v T
	if err := yaml.Unmarshal([]byte("secret: true"), &v); err != nil {
		t.Fatalf("Unmarshal エラー: %v", err)
	}
	if v.Provider != nil {
		t.Errorf("provider フィールド未指定なのに非 nil: %v", v.Provider)
	}
}
