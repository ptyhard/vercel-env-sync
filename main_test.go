package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// --- buildInitYAML のテスト ---

func TestBuildInitYAML_ContainsDefaults(t *testing.T) {
	yaml := buildInitYAML([]string{"FOO", "BAR"})
	if !strings.Contains(yaml, "defaults:") {
		t.Error("defaults: セクションが含まれていない")
	}
	if !strings.Contains(yaml, "target: [production, preview]") {
		t.Error("defaults の target が含まれていない")
	}
	if !strings.Contains(yaml, "type: sensitive") {
		t.Error("defaults の type: sensitive が含まれていない")
	}
}

func TestBuildInitYAML_NextPublicIsEncrypted(t *testing.T) {
	yaml := buildInitYAML([]string{"NEXT_PUBLIC_API_URL", "NEXT_PUBLIC_SITE"})
	if !strings.Contains(yaml, "NEXT_PUBLIC_API_URL: { type: encrypted }") {
		t.Errorf("NEXT_PUBLIC_API_URL が encrypted になっていない:\n%s", yaml)
	}
	if !strings.Contains(yaml, "NEXT_PUBLIC_SITE: { type: encrypted }") {
		t.Errorf("NEXT_PUBLIC_SITE が encrypted になっていない:\n%s", yaml)
	}
}

func TestBuildInitYAML_OtherKeyIsSensitive(t *testing.T) {
	yaml := buildInitYAML([]string{"DATABASE_URL", "SECRET_KEY"})
	if !strings.Contains(yaml, "DATABASE_URL: { type: sensitive }") {
		t.Errorf("DATABASE_URL が sensitive になっていない:\n%s", yaml)
	}
	if !strings.Contains(yaml, "SECRET_KEY: { type: sensitive }") {
		t.Errorf("SECRET_KEY が sensitive になっていない:\n%s", yaml)
	}
}

func TestBuildInitYAML_MixedKeys(t *testing.T) {
	yaml := buildInitYAML([]string{"DATABASE_URL", "NEXT_PUBLIC_API_BASE_URL", "DEBUG"})
	if !strings.Contains(yaml, "NEXT_PUBLIC_API_BASE_URL: { type: encrypted }") {
		t.Errorf("NEXT_PUBLIC_ プレフィックスが encrypted にならない:\n%s", yaml)
	}
	if !strings.Contains(yaml, "DATABASE_URL: { type: sensitive }") {
		t.Errorf("DATABASE_URL が sensitive にならない:\n%s", yaml)
	}
	if !strings.Contains(yaml, "DEBUG: { type: sensitive }") {
		t.Errorf("DEBUG が sensitive にならない:\n%s", yaml)
	}
}

func TestBuildInitYAML_NoValuesIncluded(t *testing.T) {
	// 値を含まないことを確認（実際には buildInitYAML は keys だけを受け取るので値は入れようがないが念のため）
	yaml := buildInitYAML([]string{"API_KEY", "DATABASE_URL"})
	// yamlに = や : の後に実際の値が含まれないことを確認
	// キー名と type/sensitive/encrypted 以外の文字列が入っていないことを確認
	if strings.Contains(yaml, "postgres://") || strings.Contains(yaml, "https://") {
		t.Error("yaml に値が含まれている")
	}
}

func TestBuildInitYAML_ZeroKeys(t *testing.T) {
	yaml := buildInitYAML([]string{})
	if !strings.Contains(yaml, "variables:") {
		t.Error("variables: セクションが含まれていない")
	}
	// 0件の場合は例コメントが含まれること
	if !strings.Contains(yaml, "#") {
		t.Error("0件の場合に例コメントが含まれていない")
	}
}

func TestBuildInitYAML_ContainsVariablesSection(t *testing.T) {
	yaml := buildInitYAML([]string{"FOO"})
	if !strings.Contains(yaml, "variables:") {
		t.Error("variables: セクションが含まれていない")
	}
}

func TestBuildInitYAML_ContainsWarningComment(t *testing.T) {
	yaml := buildInitYAML([]string{"FOO"})
	// 雛形であることの注意コメントが含まれること
	if !strings.Contains(yaml, "雛形") || !strings.Contains(yaml, "見直す") {
		t.Errorf("注意コメントが含まれていない:\n%s", yaml)
	}
}

// --- parseInitFlags のテスト ---

func TestParseInitFlags_Defaults(t *testing.T) {
	opts := parseInitFlags([]string{})
	if opts.env != ".env" {
		t.Errorf("env のデフォルト値が異なる: got %q", opts.env)
	}
	if opts.def != "vercel-env.yaml" {
		t.Errorf("def のデフォルト値が異なる: got %q", opts.def)
	}
	if opts.force {
		t.Error("force のデフォルト値が true になっている")
	}
}

func TestParseInitFlags_EnvFlag(t *testing.T) {
	opts := parseInitFlags([]string{"--env", ".env.production"})
	if opts.env != ".env.production" {
		t.Errorf("--env の値が反映されない: got %q", opts.env)
	}
}

func TestParseInitFlags_EnvEqualFlag(t *testing.T) {
	opts := parseInitFlags([]string{"--env=.env.production"})
	if opts.env != ".env.production" {
		t.Errorf("--env= の値が反映されない: got %q", opts.env)
	}
}

func TestParseInitFlags_DefFlag(t *testing.T) {
	opts := parseInitFlags([]string{"--def", "custom.yaml"})
	if opts.def != "custom.yaml" {
		t.Errorf("--def の値が反映されない: got %q", opts.def)
	}
}

func TestParseInitFlags_DefEqualFlag(t *testing.T) {
	opts := parseInitFlags([]string{"--def=custom.yaml"})
	if opts.def != "custom.yaml" {
		t.Errorf("--def= の値が反映されない: got %q", opts.def)
	}
}

func TestParseInitFlags_ForceFlag(t *testing.T) {
	opts := parseInitFlags([]string{"--force"})
	if !opts.force {
		t.Error("--force が有効にならない")
	}
}

func TestParseInitFlags_MultipleFlags(t *testing.T) {
	opts := parseInitFlags([]string{"--env", ".env.staging", "--def=out.yaml", "--force"})
	if opts.env != ".env.staging" {
		t.Errorf("env が間違っている: got %q", opts.env)
	}
	if opts.def != "out.yaml" {
		t.Errorf("def が間違っている: got %q", opts.def)
	}
	if !opts.force {
		t.Error("force が有効にならない")
	}
}

// --- runInit の統合テスト（一時ディレクトリを利用）---

func TestRunInit_CreatesYAML(t *testing.T) {
	dir := t.TempDir()

	envFile := filepath.Join(dir, ".env")
	defFile := filepath.Join(dir, "vercel-env.yaml")

	if err := os.WriteFile(envFile, []byte("DATABASE_URL=postgres://x\nNEXT_PUBLIC_API=https://y\nDEBUG=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runInit([]string{"--env", envFile, "--def", defFile})
	if err != nil {
		t.Fatalf("runInit でエラー: %v", err)
	}

	data, err := os.ReadFile(defFile)
	if err != nil {
		t.Fatalf("出力ファイルが存在しない: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "DATABASE_URL") {
		t.Error("DATABASE_URL が出力に含まれていない")
	}
	if !strings.Contains(content, "NEXT_PUBLIC_API") {
		t.Error("NEXT_PUBLIC_API が出力に含まれていない")
	}
	// 値が含まれないこと
	if strings.Contains(content, "postgres://x") {
		t.Error("値 postgres://x が出力に含まれている（NG）")
	}
	if strings.Contains(content, "https://y") {
		t.Error("値 https://y が出力に含まれている（NG）")
	}
}

func TestRunInit_OverwriteProtection(t *testing.T) {
	dir := t.TempDir()

	envFile := filepath.Join(dir, ".env")
	defFile := filepath.Join(dir, "vercel-env.yaml")

	if err := os.WriteFile(envFile, []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 先に defFile を作成しておく
	if err := os.WriteFile(defFile, []byte("existing content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// --force なしで実行 → エラーになるはず
	err := runInit([]string{"--env", envFile, "--def", defFile})
	if err == nil {
		t.Fatal("--force なしで既存ファイルが上書きされてしまった（エラーが返らなかった）")
	}
	if !strings.Contains(err.Error(), "上書きするには --force") {
		t.Errorf("エラーメッセージが想定と異なる: %v", err)
	}

	// ファイルの中身が変わっていないこと
	data, _ := os.ReadFile(defFile)
	if string(data) != "existing content" {
		t.Error("上書き保護が機能していない: ファイルの中身が変わっている")
	}
}

func TestRunInit_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()

	envFile := filepath.Join(dir, ".env")
	defFile := filepath.Join(dir, "vercel-env.yaml")

	if err := os.WriteFile(envFile, []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defFile, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runInit([]string{"--env", envFile, "--def", defFile, "--force"})
	if err != nil {
		t.Fatalf("--force 付きで runInit がエラー: %v", err)
	}

	data, err := os.ReadFile(defFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "old content" {
		t.Error("--force 付きでも上書きされていない")
	}
}

func TestRunInit_EnvFileNotFound(t *testing.T) {
	dir := t.TempDir()
	err := runInit([]string{"--env", filepath.Join(dir, "nonexistent.env")})
	if err == nil {
		t.Error("存在しない env ファイルでエラーが返らなかった")
	}
}

func TestRunInit_CommentsNotValues(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	defFile := filepath.Join(dir, "out.yaml")

	// コメント行を含む .env
	if err := os.WriteFile(envFile, []byte("# これはコメント\nSECRET_KEY=supersecret\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInit([]string{"--env", envFile, "--def", defFile}); err != nil {
		t.Fatalf("runInit エラー: %v", err)
	}

	data, _ := os.ReadFile(defFile)
	content := string(data)

	if strings.Contains(content, "supersecret") {
		t.Error("値 supersecret が yaml に混入している（NG）")
	}
	if !strings.Contains(content, "SECRET_KEY") {
		t.Error("SECRET_KEY がキーとして含まれていない")
	}
}

// --- 往復（round-trip）統合テスト ---
// init が生成した YAML が、従来の同期パスと同じ definition 構造体へ
// そのまま読み込め、type / target が想定どおり解決されることを保証する。
// acceptanceCriteria「生成物がそのまま既存パーサで読める正しい形式」の担保。

func TestInitYAML_RoundTripParsesAsDefinition(t *testing.T) {
	keys := []string{"DATABASE_URL", "DEBUG", "NEXT_PUBLIC_API"}
	out := buildInitYAML(keys)

	var def definition
	if err := yaml.Unmarshal([]byte(out), &def); err != nil {
		t.Fatalf("生成 YAML が既存パーサで読めない: %v", err)
	}

	// defaults が既存 run() のデフォルトと揃っているか
	if def.Defaults.Type != "sensitive" {
		t.Errorf("defaults.type = %q, want sensitive", def.Defaults.Type)
	}
	if got := normalizeTarget(def.Defaults.Target); len(got) != 2 || got[0] != "production" || got[1] != "preview" {
		t.Errorf("defaults.target = %v, want [production preview]", got)
	}

	// 全キーが variables に列挙されているか
	for _, k := range keys {
		if _, ok := def.Variables[k]; !ok {
			t.Errorf("variables に %s が含まれていない", k)
		}
	}

	// type サジェスト: NEXT_PUBLIC_ → encrypted、他 → sensitive
	cases := map[string]string{
		"NEXT_PUBLIC_API": "encrypted",
		"DATABASE_URL":    "sensitive",
		"DEBUG":           "sensitive",
	}
	for key, wantType := range cases {
		if got := def.Variables[key].Type; got != wantType {
			t.Errorf("%s の type = %q, want %q", key, got, wantType)
		}
	}
}

// init 生成 → 同期パスの type/target 検証（validTypes/validTargets）まで通ることを確認する。
// 不正な type/target を生成していれば run() 側でエラーになるため、その手前を機械的に検証する。
func TestInitYAML_RoundTripTypesAndTargetsAreValid(t *testing.T) {
	out := buildInitYAML([]string{"NEXT_PUBLIC_FOO", "BAR_SECRET"})

	var def definition
	if err := yaml.Unmarshal([]byte(out), &def); err != nil {
		t.Fatalf("生成 YAML のパースに失敗: %v", err)
	}

	defaultTarget := normalizeTarget(def.Defaults.Target)
	for key, conf := range def.Variables {
		typ := conf.Type
		if typ == "" {
			typ = def.Defaults.Type
		}
		if !validTypes[typ] {
			t.Errorf("%s: 生成された type %q が validTypes に無い", key, typ)
		}
		target := normalizeTarget(conf.Target)
		if len(target) == 0 {
			target = defaultTarget
		}
		for _, tg := range target {
			if !validTargets[tg] {
				t.Errorf("%s: 生成された target %q が validTargets に無い", key, tg)
			}
		}
	}
}
