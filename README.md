# vercel-env-sync

定義ファイル `vercel-env.yaml` で宣言した環境変数を Vercel へ **一括登録（同期）** する Go 製 CLI。
Vercel REST API (`POST /v10/projects/{id}/env?upsert=true`) を使うため、再実行すると既存の変数は **更新（upsert）** される。

[ptyhard/arg-next の `scripts/vercel-env-push.mjs`](../arg-next/scripts/vercel-env-push.mjs) と同じ仕様を Go で実装したもの。

## 仕組み

- **type / target は `vercel-env.yaml` で明示的に宣言**する（キー名のヒューリスティックに頼らない）。
- **値は `vercel-env.yaml` には書かない**（git にコミットされるため）。値は `.env(.production)` から取得する。
- 定義に無いキーは登録されない（`.env` にあっても警告のうえスキップ）。
- 定義にあるが `.env` に値が無いキーも警告のうえスキップ。

## インストール

### Homebrew（macOS / Linux）

Cask は専用 tap リポジトリではなく本リポジトリの `Casks/` に同梱しているため、初回のみ tap を URL 指定で追加します。

```bash
brew tap ptyhard/vercel-env-sync https://github.com/ptyhard/vercel-env-sync
brew install ptyhard/vercel-env-sync/vercel-env-sync
```

> GoReleaser v2.16 以降は formula が廃止されたため、配布は Homebrew **Cask** で行います。リポジトリ名が `homebrew-` で始まらないので、`brew tap` には上記のように URL を明示します。

### go install

```bash
go install github.com/ptyhard/vercel-env-sync@latest
```

### バイナリを直接ダウンロード

[GitHub Releases](https://github.com/ptyhard/vercel-env-sync/releases) から最新バイナリをダウンロードしてください。
darwin/amd64、darwin/arm64、linux/amd64、linux/arm64 向けアーカイブが提供されます。

### ソースからビルド

```bash
go build -o vercel-env-sync .
```

## リリース（メンテナ向け）

`v*` タグを push すると [.github/workflows/release.yml](.github/workflows/release.yml) が GoReleaser を実行し、GitHub Release の作成と本リポジトリ `Casks/` への cask 生成・コミットを行います。

```bash
git tag v0.1.0
git push origin v0.1.0
```

cask は本リポジトリに同梱するため、push には Actions が自動提供する `GITHUB_TOKEN` のみを使います。**追加の Secrets 設定は不要**です（`contents: write` 権限はワークフロー内で付与済み）。

> **注意**: main ブランチにブランチ保護で push 制限をかけている場合は、GitHub Actions（`github-actions[bot]`）による push を許可する設定が必要です。

## 定義ファイル `vercel-env.yaml`

```yaml
defaults:
  target: [production, preview]
  type: sensitive
variables:
  NEXT_PUBLIC_FIREBASE_API_KEY: { type: encrypted }
  DATABASE_URL: { type: sensitive, target: [production] }
```

| type | Vercel UI | 説明 |
|------|-----------|------|
| `encrypted` | Variables | ダッシュボードで値を再表示できる通常の変数 |
| `sensitive` | Sensitive | 保存後は値を読めないシークレット向け |
| `plain` | Plain | 暗号化なしの平文 |

`target` は `production` / `preview` / `development` の配列。各変数で省略すると `defaults` が使われる。

## 事前準備

```bash
# 1. プロジェクトをリンク（初回のみ。.vercel/project.json が生成される）
vercel link

# 2. アクセストークンを発行
#    https://vercel.com/account/tokens
```

## 使い方

```bash
# dry-run（送信せず key / type / target だけ確認。値は表示されない）
VERCEL_PROJECT_ID=dummy ./vercel-env-sync --env .env.production --dry-run

# 本番投入（登録対象を一覧表示し、y/N の確認のうえ送信する）
VERCEL_TOKEN=xxxxx ./vercel-env-sync --env .env.production

# 確認をスキップ（CI など）
VERCEL_TOKEN=xxxxx ./vercel-env-sync --env .env.production --yes
```

送信前に登録対象（key / type / target）を一覧表示し、既存変数は upsert で上書きされるため `y/N` の確認を取る。`--yes`（`-y`）で確認をスキップできる。非対話環境で `--yes` を付けないと安全のため中止する。

## オプション / 環境変数

| 項目 | 必須 | 説明 |
|------|------|------|
| `--env <file>` | – | 値を読む env ファイル（デフォルト `.env`） |
| `--def <file>` | – | type/target 定義 YAML（デフォルト `vercel-env.yaml`） |
| `--dry-run` | – | 送信せず登録予定のみ表示 |
| `--yes` / `-y` | – | 送信前の確認をスキップ |
| `VERCEL_TOKEN` | ◯ | Vercel アクセストークン（dry-run 時は不要） |
| `VERCEL_PROJECT_ID` | △ | プロジェクト ID。未指定なら `.vercel/project.json` から自動取得 |
| `VERCEL_TEAM_ID` | – | チーム(Org) ID。未指定なら `.vercel/project.json` の `orgId` |
