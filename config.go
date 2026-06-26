package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProviderVal はYAMLで string か []string を受け付ける provider フィールドの型。
// `provider: vercel` でも `provider: [vercel, github]` でも解析できる。
type ProviderVal struct {
	Values []string
}

func (p *ProviderVal) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		p.Values = []string{value.Value}
	case yaml.SequenceNode:
		var ss []string
		if err := value.Decode(&ss); err != nil {
			return err
		}
		p.Values = ss
	default:
		return fmt.Errorf("provider は文字列または文字列の配列で指定してください")
	}
	return nil
}

const apiBase = "https://api.vercel.com"

// options はコマンドラインフラグの値を保持する。
type options struct {
	env      string
	def      string
	dryRun   bool
	yes      bool
	provider string
}

// varConf は定義 YAML の variables 配下 1 件分の設定。
type varConf struct {
	Secret       *bool        `yaml:"secret"`
	Environments []string     `yaml:"environments"`
	Provider     *ProviderVal `yaml:"provider"`
}

// definition は定義 YAML 全体の構造。
type definition struct {
	Defaults struct {
		Secret       *bool        `yaml:"secret"`
		Environments []string     `yaml:"environments"`
		Provider     *ProviderVal `yaml:"provider"`
	} `yaml:"defaults"`
	Variables map[string]varConf `yaml:"variables"`
}

// parseFlags はコマンドライン引数を解析する。flag パッケージは特殊な
// 短縮形 (-y) と長形 (--yes) の両立や --dry-run の扱いが煩雑なため手で処理する。
func parseFlags(argv []string) options {
	opts := options{env: ".env", def: "env-sync.yaml", provider: "vercel"}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		next := func() string {
			i++
			if i >= len(argv) {
				fmt.Fprintf(os.Stderr, "エラー: %s には値が必要です\n", arg)
				os.Exit(1)
			}
			return argv[i]
		}
		switch {
		case arg == "--env" || arg == "-env":
			opts.env = next()
		case strings.HasPrefix(arg, "--env="):
			opts.env = strings.TrimPrefix(arg, "--env=")
		case arg == "--def" || arg == "-def":
			opts.def = next()
		case strings.HasPrefix(arg, "--def="):
			opts.def = strings.TrimPrefix(arg, "--def=")
		case arg == "--dry-run" || arg == "-dry-run":
			opts.dryRun = true
		case arg == "--yes" || arg == "-yes" || arg == "-y":
			opts.yes = true
		case arg == "--version" || arg == "-version":
			fmt.Printf("env-sync version %s (commit: %s, built: %s)\n", version, commit, date)
			os.Exit(0)
		case arg == "--provider" || arg == "-provider":
			v := next()
			if !isRegisteredProvider(v) {
				names := strings.Join(registeredProviderNames(), " または ")
				fmt.Fprintf(os.Stderr, "エラー: --provider には %s を指定してください\n", names)
				os.Exit(1)
			}
			opts.provider = v
		case strings.HasPrefix(arg, "--provider="):
			v := strings.TrimPrefix(arg, "--provider=")
			if !isRegisteredProvider(v) {
				names := strings.Join(registeredProviderNames(), " または ")
				fmt.Fprintf(os.Stderr, "エラー: --provider には %s を指定してください\n", names)
				os.Exit(1)
			}
			opts.provider = v
		case arg == "-h" || arg == "--help":
			printUsage()
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "エラー: 不明な引数: %s\n", arg)
			printUsage()
			os.Exit(1)
		}
	}
	return opts
}

// isRegisteredProvider は name が registry に登録されているかを返す。
func isRegisteredProvider(name string) bool {
	_, ok := providerRegistry[name]
	return ok
}
