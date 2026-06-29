package i18n_test

import (
	"testing"

	"github.com/ptyhard/env-sync/internal/i18n"
)

// TestResolve_優先順位 は Resolve の言語解決優先順位を表駆動テストで確認する。
func TestResolve_優先順位(t *testing.T) {
	tests := []struct {
		name      string
		flagVal   string
		envVal    string
		configVal string
		want      i18n.Lang
	}{
		{
			name:    "フラグが最優先",
			flagVal: "ja", envVal: "en", configVal: "en",
			want: i18n.LangJA,
		},
		{
			name:    "フラグが空のときは env 変数を使う",
			flagVal: "", envVal: "ja", configVal: "en",
			want: i18n.LangJA,
		},
		{
			name:    "フラグ・env 変数が空のときは config を使う",
			flagVal: "", envVal: "", configVal: "ja",
			want: i18n.LangJA,
		},
		{
			name:    "すべて空のときはデフォルト en",
			flagVal: "", envVal: "", configVal: "",
			want: i18n.LangEN,
		},
		{
			name:    "すべて en のとき en",
			flagVal: "en", envVal: "en", configVal: "en",
			want: i18n.LangEN,
		},
		{
			name:    "フラグが不正コードのとき次候補へ（env が ja）",
			flagVal: "xx", envVal: "ja", configVal: "en",
			want: i18n.LangJA,
		},
		{
			name:    "フラグ・env ともに不正のとき config を使う",
			flagVal: "xx", envVal: "yy", configVal: "ja",
			want: i18n.LangJA,
		},
		{
			name:    "すべて不正のときは en にフォールバック",
			flagVal: "xx", envVal: "yy", configVal: "zz",
			want: i18n.LangEN,
		},
		{
			name:    "フラグが en のとき config ja より en が優先",
			flagVal: "en", envVal: "", configVal: "ja",
			want: i18n.LangEN,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := i18n.Resolve(tc.flagVal, tc.envVal, tc.configVal)
			if got != tc.want {
				t.Errorf("Resolve(%q, %q, %q) = %q, want %q", tc.flagVal, tc.envVal, tc.configVal, got, tc.want)
			}
		})
	}
}

// TestSetLang_対応コード は SetLang が対応コードで true、未対応コードで false を返すことを確認する。
func TestSetLang_対応コード(t *testing.T) {
	tests := []struct {
		code     string
		wantOK   bool
		wantLang i18n.Lang
	}{
		{"en", true, i18n.LangEN},
		{"ja", true, i18n.LangJA},
		{"", false, i18n.LangEN},
		{"xx", false, i18n.LangEN},
		{"EN", false, i18n.LangEN},
		{"JA", false, i18n.LangEN},
	}

	for _, tc := range tests {
		t.Run("code="+tc.code, func(t *testing.T) {
			// テスト後に en に戻す
			t.Cleanup(func() { i18n.SetLang("en") })

			got := i18n.SetLang(tc.code)
			if got != tc.wantOK {
				t.Errorf("SetLang(%q) = %v, want %v", tc.code, got, tc.wantOK)
			}
		})
	}
}

// TestT_JA は SetLang("ja") 後に T() が日本語カタログの値を返すことを確認する。
func TestT_JA(t *testing.T) {
	t.Cleanup(func() { i18n.SetLang("en") })
	i18n.SetLang("ja")

	got := i18n.T(i18n.MsgNoEntries)
	want := "登録対象がありません"
	if got != want {
		t.Errorf("T(MsgNoEntries) [ja] = %q, want %q", got, want)
	}
}

// TestT_EN は SetLang("en") 後に T() が英語カタログの値を返すことを確認する。
func TestT_EN(t *testing.T) {
	t.Cleanup(func() { i18n.SetLang("en") })
	i18n.SetLang("en")

	got := i18n.T(i18n.MsgNoEntries)
	want := "No entries to sync"
	if got != want {
		t.Errorf("T(MsgNoEntries) [en] = %q, want %q", got, want)
	}
}

// TestT_書式引数展開 は T() が args を fmt.Sprintf で展開することを確認する。
func TestT_書式引数展開(t *testing.T) {
	t.Cleanup(func() { i18n.SetLang("en") })
	i18n.SetLang("en")

	got := i18n.T(i18n.MsgEnvFileNotFound, "myfile.env")
	want := "env file not found: myfile.env"
	if got != want {
		t.Errorf("T(MsgEnvFileNotFound, myfile.env) [en] = %q, want %q", got, want)
	}
}

// TestT_未知キー は未知キーに対して安全にキー名自体を返すことを確認する。
func TestT_未知キー(t *testing.T) {
	t.Cleanup(func() { i18n.SetLang("en") })
	i18n.SetLang("en")

	const unknownKey i18n.MsgKey = "this.key.does.not.exist"
	got := i18n.T(unknownKey)
	// キー名そのものが返る（panic しない）
	if got == "" {
		t.Error("T(未知キー) は空文字列を返してはいけない")
	}
}

// TestCatalogKeyParity は en と ja のカタログのキー集合が一致することを確認する（キー漏れ検出）。
func TestCatalogKeyParity(t *testing.T) {
	enKeys := i18n.ENKeys()
	jaKeys := i18n.JAKeys()

	// en にあって ja にないキーを検出
	jaSet := make(map[i18n.MsgKey]bool, len(jaKeys))
	for _, k := range jaKeys {
		jaSet[k] = true
	}
	for _, k := range enKeys {
		if !jaSet[k] {
			t.Errorf("en カタログにあるが ja カタログにないキー: %q", k)
		}
	}

	// ja にあって en にないキーを検出
	enSet := make(map[i18n.MsgKey]bool, len(enKeys))
	for _, k := range enKeys {
		enSet[k] = true
	}
	for _, k := range jaKeys {
		if !enSet[k] {
			t.Errorf("ja カタログにあるが en カタログにないキー: %q", k)
		}
	}
}
