// Package i18n はCLIメッセージの国際化（i18n）基盤を提供する。
// 他の internal パッケージへの依存は持たない（循環 import 防止）。
package i18n

import "fmt"

// Lang は表示言語を表す型。
type Lang string

const (
	// LangEN は英語。
	LangEN Lang = "en"
	// LangJA は日本語。
	LangJA Lang = "ja"
)

// current は現在の表示言語。デフォルトは英語。
var current = LangEN

// catalogs は言語コードからカタログへのマップ。
var catalogs = map[Lang]map[MsgKey]string{
	LangEN: enCatalog,
	LangJA: jaCatalog,
}

// SetLang は表示言語を code に設定する。
// 対応コード（"en" / "ja"）のときは true を返す。
// 未対応または空文字のときは en にフォールバックして false を返す。
func SetLang(code string) bool {
	l := Lang(code)
	if _, ok := catalogs[l]; ok {
		current = l
		return true
	}
	current = LangEN
	return false
}

// Resolve は flagVal > envVal > configVal > "en" の優先順位で言語を決定する純粋関数。
// 各候補を順に評価し、最初の「非空かつ対応済みコード」を採用する。
// すべての候補が非対応または空のときは LangEN を返す。
func Resolve(flagVal, envVal, configVal string) Lang {
	for _, code := range []string{flagVal, envVal, configVal} {
		if code == "" {
			continue
		}
		l := Lang(code)
		if _, ok := catalogs[l]; ok {
			return l
		}
	}
	return LangEN
}

// T は現在の言語カタログから key に対応するメッセージを返す。
// args が指定された場合は fmt.Sprintf で書式展開する。
// キーが見つからない場合は en カタログを試み、それも無ければキー名をそのまま返す（panic しない）。
func T(key MsgKey, args ...any) string {
	catalog := catalogs[current]
	msg, ok := catalog[key]
	if !ok {
		// en カタログにフォールバック
		msg, ok = enCatalog[key]
		if !ok {
			// 最終フォールバック: キー名を返す
			return string(key)
		}
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// ENKeys は en カタログのキー一覧を返す（テスト・検査用）。
func ENKeys() []MsgKey {
	keys := make([]MsgKey, 0, len(enCatalog))
	for k := range enCatalog {
		keys = append(keys, k)
	}
	return keys
}

// JAKeys は ja カタログのキー一覧を返す（テスト・検査用）。
func JAKeys() []MsgKey {
	keys := make([]MsgKey, 0, len(jaCatalog))
	for k := range jaCatalog {
		keys = append(keys, k)
	}
	return keys
}
