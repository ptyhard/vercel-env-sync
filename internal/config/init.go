package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/ptyhard/env-sync/internal/i18n"
)

// InitOptions は init サブコマンドのフラグ値を保持する。
type InitOptions struct {
	Env   string
	Def   string
	Force bool
}

// ParseInitFlags は init サブコマンドのコマンドライン引数を解析する。
func ParseInitFlags(argv []string, printUsageFn func()) InitOptions {
	opts := InitOptions{Env: ".env", Def: "env-sync.yaml"}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		next := func() string {
			i++
			if i >= len(argv) {
				fmt.Fprint(os.Stderr, i18n.T(i18n.MsgFlagNeedsValue, arg))
				os.Exit(1)
			}
			return argv[i]
		}
		// requireValue は空文字のパス指定（例: --env=）を弾く。
		requireValue := func(flag, v string) string {
			if v == "" {
				fmt.Fprint(os.Stderr, i18n.T(i18n.MsgFlagNeedsNonEmpty, flag))
				os.Exit(1)
			}
			return v
		}
		switch {
		case arg == "--env" || arg == "-env":
			opts.Env = requireValue("--env", next())
		case strings.HasPrefix(arg, "--env="):
			opts.Env = requireValue("--env", strings.TrimPrefix(arg, "--env="))
		case arg == "--def" || arg == "-def":
			opts.Def = requireValue("--def", next())
		case strings.HasPrefix(arg, "--def="):
			opts.Def = requireValue("--def", strings.TrimPrefix(arg, "--def="))
		case arg == "--force" || arg == "-force":
			opts.Force = true
		case arg == "--lang" || arg == "-lang" || arg == "--language" || arg == "-language":
			// 言語フラグは main.go でプレスキャン済みのため値を読み飛ばす。
			next()
		case strings.HasPrefix(arg, "--lang=") || strings.HasPrefix(arg, "--language="):
			// 言語フラグは main.go でプレスキャン済みのためスキップする。
		case arg == "-h" || arg == "--help":
			if printUsageFn != nil {
				printUsageFn()
			}
			os.Exit(0)
		default:
			fmt.Fprint(os.Stderr, i18n.T(i18n.MsgFlagUnknown, arg))
			if printUsageFn != nil {
				printUsageFn()
			}
			os.Exit(1)
		}
	}
	return opts
}

// BuildInitYAML は keys から env-sync.yaml の雛形テキストを生成する。
// 値は一切含まない。yaml.Marshal は使わず手組みテキスト生成でコメントを差し込む。
func BuildInitYAML(keys []string) string {
	var sb strings.Builder

	sb.WriteString(i18n.T(i18n.MsgInitYAMLHeader))
	sb.WriteString("\n")
	sb.WriteString("defaults:\n")
	sb.WriteString("  secret: true\n")
	sb.WriteString("\n")
	sb.WriteString("variables:\n")

	if len(keys) == 0 {
		sb.WriteString(i18n.T(i18n.MsgInitYAMLExample))
		return sb.String()
	}

	for _, key := range keys {
		var secret string
		if strings.HasPrefix(key, "NEXT_PUBLIC_") {
			secret = "false"
		} else {
			secret = "true"
		}
		sb.WriteString("  ")
		sb.WriteString(yamlKey(key))
		sb.WriteString(": { secret: ")
		sb.WriteString(secret)
		sb.WriteString(" }\n")
	}

	return sb.String()
}

// yamlKey は YAML のマップキーとして安全に出力できる形に整える。
// 環境変数名として一般的な ^[A-Za-z_][A-Za-z0-9_]*$ はそのまま、
// それ以外（空白や : を含む等）は単一引用符でクォートして
// 生成 YAML がパース不能にならないようにする。
func yamlKey(key string) string {
	if isSafeYAMLKey(key) {
		return key
	}
	// 単一引用符内のリテラル ' は '' でエスケープする。
	return "'" + strings.ReplaceAll(key, "'", "''") + "'"
}

// isSafeYAMLKey は key が ^[A-Za-z_][A-Za-z0-9_]*$ に一致するかを返す。
func isSafeYAMLKey(key string) bool {
	if key == "" {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		isLetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
		isDigit := c >= '0' && c <= '9'
		if i == 0 {
			if !isLetter {
				return false
			}
		} else if !isLetter && !isDigit {
			return false
		}
	}
	return true
}

// RunInit は init サブコマンドのメイン処理。
func RunInit(argv []string, printUsageFn func()) error {
	opts := ParseInitFlags(argv, printUsageFn)

	// os.ReadFile のエラーで分岐する。FileExists での事前チェックは
	// 権限エラー等を「見つかりません」と誤判定し得るため使わない。
	envText, err := os.ReadFile(opts.Env)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s", i18n.T(i18n.MsgEnvFileNotFound, opts.Env))
		}
		return fmt.Errorf("%s", i18n.T(i18n.MsgInitEnvFileReadFail, opts.Env, err))
	}
	envVars := ParseDotenv(string(envText))
	keys := SortedStrKeys(envVars)

	text := BuildInitYAML(keys)

	// 上書き保護は O_CREATE|O_EXCL でアトミックに行う。
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !opts.Force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	f, err := os.OpenFile(opts.Def, flags, 0o644)
	if err != nil {
		if !opts.Force && os.IsExist(err) {
			return fmt.Errorf("%s", i18n.T(i18n.MsgFileExists, opts.Def))
		}
		return fmt.Errorf("%s", i18n.T(i18n.MsgInitDefWriteFail, opts.Def, err))
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return fmt.Errorf("%s", i18n.T(i18n.MsgInitDefWriteFail, opts.Def, err))
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("%s", i18n.T(i18n.MsgInitDefWriteFail, opts.Def, err))
	}

	fmt.Print(i18n.T(i18n.MsgGenerated, opts.Def))
	fmt.Print(i18n.T(i18n.MsgInitKeyCount, len(keys)))
	if len(keys) > 0 {
		fmt.Print(i18n.T(i18n.MsgInitKeyListHeader))
		for _, k := range keys {
			fmt.Printf("  %s\n", k)
		}
	}
	fmt.Println()
	fmt.Println(i18n.T(i18n.MsgInitSecretNote))

	return nil
}
