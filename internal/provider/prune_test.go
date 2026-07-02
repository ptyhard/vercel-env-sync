package provider_test

import (
	"testing"

	"github.com/ptyhard/env-sync/internal/provider"
)

// --- Options.PruneKeep のテスト ---

func TestPruneKeep_DefinedKeyIsKept(t *testing.T) {
	opts := provider.Options{DefinedKeys: []string{"API_KEY", "DB_URL"}}
	keep := opts.PruneKeep()
	if !keep("API_KEY") {
		t.Error("定義済みキー API_KEY は保持されるべき")
	}
	if keep("UNDEFINED_KEY") {
		t.Error("未定義キー UNDEFINED_KEY は保持されないべき")
	}
}

func TestPruneKeep_CaseInsensitive(t *testing.T) {
	// GitHub は Secret/Variable 名を大文字で保持するため、大文字小文字を区別せず保持判定する
	opts := provider.Options{DefinedKeys: []string{"api_key"}}
	keep := opts.PruneKeep()
	if !keep("API_KEY") {
		t.Error("定義済みキーの大文字表記 API_KEY は保持されるべき")
	}
}

func TestPruneKeep_ExcludePattern(t *testing.T) {
	opts := provider.Options{
		DefinedKeys:  []string{"API_KEY"},
		PruneExclude: []string{"BLOB_*", "SENTRY_DSN"},
	}
	keep := opts.PruneKeep()
	cases := []struct {
		key  string
		want bool
	}{
		{"BLOB_READ_WRITE_TOKEN", true}, // glob パターン一致
		{"SENTRY_DSN", true},            // 完全一致パターン
		{"sentry_dsn", true},            // パターンも大文字小文字非区別
		{"API_KEY", true},               // 定義済みキー
		{"OTHER_KEY", false},            // どれにも該当しない
	}
	for _, tc := range cases {
		if got := keep(tc.key); got != tc.want {
			t.Errorf("keep(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestPruneKeep_NoPatterns(t *testing.T) {
	opts := provider.Options{}
	keep := opts.PruneKeep()
	if keep("ANY_KEY") {
		t.Error("定義もパターンも無いとき ANY_KEY は保持されないべき")
	}
}

// --- ValidatePrunePatterns のテスト ---

func TestValidatePrunePatterns_Valid(t *testing.T) {
	if invalid, ok := provider.ValidatePrunePatterns([]string{"BLOB_*", "FOO?", "[AB]_KEY"}); !ok {
		t.Errorf("正しいパターンで invalid=%q が返った", invalid)
	}
}

func TestValidatePrunePatterns_Invalid(t *testing.T) {
	invalid, ok := provider.ValidatePrunePatterns([]string{"BLOB_*", "[unclosed"})
	if ok {
		t.Fatal("不正なパターン [unclosed でエラーになるべき")
	}
	if invalid != "[unclosed" {
		t.Errorf("invalid = %q, want [unclosed", invalid)
	}
}

func TestValidatePrunePatterns_Empty(t *testing.T) {
	if _, ok := provider.ValidatePrunePatterns(nil); !ok {
		t.Error("空のパターン一覧はエラーにならないべき")
	}
}
