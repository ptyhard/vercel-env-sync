package config_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ptyhard/env-sync/internal/config"
)

// --- Definition の prune / prune_exclude パースのテスト ---

func TestDefinition_PruneParse(t *testing.T) {
	text := `
prune: true
prune_exclude:
  - BLOB_*
  - SENTRY_DSN
variables:
  API_KEY: { secret: true }
`
	var def config.Definition
	if err := yaml.Unmarshal([]byte(text), &def); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if !def.Prune {
		t.Error("Prune = false, want true")
	}
	if len(def.PruneExclude) != 2 || def.PruneExclude[0] != "BLOB_*" || def.PruneExclude[1] != "SENTRY_DSN" {
		t.Errorf("PruneExclude = %v, want [BLOB_* SENTRY_DSN]", def.PruneExclude)
	}
}

func TestDefinition_PruneDefaultFalse(t *testing.T) {
	text := `
variables:
  API_KEY: { secret: true }
`
	var def config.Definition
	if err := yaml.Unmarshal([]byte(text), &def); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if def.Prune {
		t.Error("Prune = true, want false（未指定時のデフォルト）")
	}
	if def.PruneExclude != nil {
		t.Errorf("PruneExclude = %v, want nil", def.PruneExclude)
	}
}

// --- ParseFlags の --prune のテスト ---

func TestParseFlags_Prune(t *testing.T) {
	opts := config.ParseFlags([]string{"--prune"}, nil, nil)
	if !opts.Prune {
		t.Error("--prune 指定時 opts.Prune = false, want true")
	}
}

func TestParseFlags_PruneDefaultFalse(t *testing.T) {
	opts := config.ParseFlags([]string{}, nil, nil)
	if opts.Prune {
		t.Error("未指定時 opts.Prune = true, want false")
	}
}
