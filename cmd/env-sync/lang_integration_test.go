package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelpLanguage_Integration は --help 出力の言語切替を検証する。
// --lang / ENV_SYNC_LANG / config の language フィールドが
// ParseFlags の前に確定していることを実バイナリで確認する。
func TestHelpLanguage_Integration(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "env-sync-help-lang-test")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("ビルド失敗: %s\n%s", err, out)
	}

	// catalog_en.go / catalog_ja.go の MsgUsage から言語を識別できる部分文字列
	const (
		enSub = "sync environment variables" // 英語 usage に含まれる固有文字列
		jaSub = "定義ファイルで宣言した"                // 日本語 usage に含まれる固有文字列
	)

	tests := []struct {
		name       string
		args       []string // --help を含む引数
		envLang    string   // ENV_SYNC_LANG（空なら設定しない）
		configLang string   // .env-sync.config.yaml の language（空なら config を作らない）
		wantSub    string   // 出力に含まれるべき文字列
		notWant    string   // 出力に含まれてはいけない文字列
	}{
		{name: "デフォルトは英語", args: []string{"--help"}, wantSub: enSub, notWant: jaSub},
		{name: "flagでja", args: []string{"--help", "--lang", "ja"}, wantSub: jaSub, notWant: enSub},
		{name: "lang後置でも有効", args: []string{"--lang", "ja", "--help"}, wantSub: jaSub, notWant: enSub},
		{name: "envでja", args: []string{"--help"}, envLang: "ja", wantSub: jaSub, notWant: enSub},
		{name: "configでja", args: []string{"--help"}, configLang: "ja", wantSub: jaSub, notWant: enSub},
		{name: "flagがenvを上書き", args: []string{"--help", "--lang", "en"}, envLang: "ja", wantSub: enSub, notWant: jaSub},
		{name: "flagがconfigを上書き", args: []string{"--help", "--lang", "en"}, configLang: "ja", wantSub: enSub, notWant: jaSub},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			if tc.configLang != "" {
				cfg := "language: " + tc.configLang + "\n"
				if err := os.WriteFile(filepath.Join(workDir, ".env-sync.config.yaml"), []byte(cfg), 0o600); err != nil {
					t.Fatalf("config 書き込み失敗: %s", err)
				}
			}

			cmd := exec.Command(bin, tc.args...)
			cmd.Dir = workDir
			// 実ユーザーの global config（~/.config/env-sync/config.yaml）を
			// 読み込まないよう HOME / XDG_CONFIG_HOME を一時ディレクトリへ隔離する。
			env := []string{
				"HOME=" + workDir,
				"XDG_CONFIG_HOME=" + filepath.Join(workDir, ".config"),
				"PATH=" + os.Getenv("PATH"),
			}
			if tc.envLang != "" {
				env = append(env, "ENV_SYNC_LANG="+tc.envLang)
			}
			cmd.Env = env

			// --help は exit 0 で終了するため、エラーは無視して出力のみ検証する。
			out, _ := cmd.CombinedOutput()
			got := string(out)
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("出力に %q が含まれない\n出力: %q", tc.wantSub, got)
			}
			if tc.notWant != "" && strings.Contains(got, tc.notWant) {
				t.Errorf("出力に %q が含まれてはいけない\n出力: %q", tc.notWant, got)
			}
		})
	}
}

// 言語解決（i18n）の統合テスト。実バイナリをビルドし、
// フラグ(--lang) > 環境変数(ENV_SYNC_LANG) > config(language) > デフォルト en
// の優先順位と不正コードの en フォールバックを検証する。
//
// 存在しない env ファイルを指定すると run() が MsgEnvFileNotFound エラーを返し、
// main() が解決済み言語で stderr へ出力する。この出力の言語を判定材料に使う。
func TestLanguageResolution_Integration(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "env-sync-lang-test")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("ビルド失敗: %s\n%s", err, out)
	}

	const (
		enMsg = "env file not found:"
		jaMsg = "env ファイルが見つかりません:"
	)

	// 存在しない env ファイル（読み込み前の存在チェックで弾かれる）
	missingEnv := filepath.Join(t.TempDir(), "no-such.env")

	tests := []struct {
		name       string
		flagLang   string // --lang の値（空なら付けない）
		envLang    string // ENV_SYNC_LANG（空なら設定しない）
		configLang string // .env-sync.config.yaml の language（空なら config を作らない）
		wantSub    string // stderr に含まれるべき文字列
		notWant    string // stderr に含まれてはいけない文字列
	}{
		{name: "デフォルトは英語", wantSub: enMsg, notWant: jaMsg},
		{name: "configでja", configLang: "ja", wantSub: jaMsg, notWant: enMsg},
		{name: "envでja", envLang: "ja", wantSub: jaMsg, notWant: enMsg},
		{name: "flagでja", flagLang: "ja", wantSub: jaMsg, notWant: enMsg},
		{name: "envがconfigを上書き", envLang: "en", configLang: "ja", wantSub: enMsg, notWant: jaMsg},
		{name: "flagがenvを上書き", flagLang: "en", envLang: "ja", wantSub: enMsg, notWant: jaMsg},
		{name: "flagがconfigを上書き", flagLang: "en", configLang: "ja", wantSub: enMsg, notWant: jaMsg},
		{name: "不正なflagはenにフォールバック", flagLang: "xx", wantSub: enMsg, notWant: jaMsg},
		{name: "不正なflagでもconfigのjaを拾う", flagLang: "xx", configLang: "ja", wantSub: jaMsg, notWant: enMsg},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			if tc.configLang != "" {
				cfg := "language: " + tc.configLang + "\n"
				if err := os.WriteFile(filepath.Join(workDir, ".env-sync.config.yaml"), []byte(cfg), 0o600); err != nil {
					t.Fatalf("config 書き込み失敗: %s", err)
				}
			}

			args := []string{"--env", missingEnv}
			if tc.flagLang != "" {
				args = append(args, "--lang", tc.flagLang)
			}
			cmd := exec.Command(bin, args...)
			cmd.Dir = workDir
			// 実ユーザーの global config（~/.config/env-sync/config.yaml）を
			// 読み込まないよう HOME / XDG_CONFIG_HOME を一時ディレクトリへ隔離する。
			env := []string{
				"HOME=" + workDir,
				"XDG_CONFIG_HOME=" + filepath.Join(workDir, ".config"),
				"PATH=" + os.Getenv("PATH"),
			}
			if tc.envLang != "" {
				env = append(env, "ENV_SYNC_LANG="+tc.envLang)
			}
			cmd.Env = env

			// env ファイル不在で exit 1 になるため、エラーは無視して出力のみ検証する。
			out, _ := cmd.CombinedOutput()
			got := string(out)
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("出力に %q が含まれない\n出力: %q", tc.wantSub, got)
			}
			if tc.notWant != "" && strings.Contains(got, tc.notWant) {
				t.Errorf("出力に %q が含まれてはいけない\n出力: %q", tc.notWant, got)
			}
		})
	}
}
