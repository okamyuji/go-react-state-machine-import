# go-react-state-machine-import

CSVインポートジョブを題材に、**13状態 × 15イベント × 23本の遷移** を持つ状態機械をGoとReactの両方で実装したサンプルアプリケーションです。フラグ乱立を避けて状態遷移テーブル駆動の設計をそのまま学べるよう、意図的に標準ライブラリとTypeScript/React最小構成だけで書いています。

解説記事はZennに掲載しています。
**📝 [Go + Reactで現場レベルの状態遷移を1つのテーブルに統合する — 13状態×15イベントを型で閉じ込める](https://zenn.dev/okamyuji/articles/golang-react-state-machine-transition-table)**

## 何が学べるか

- フラグ列が増えていく設計がなぜ必ず組み合わせ爆発に行き着くのか
- フェーズごとに別マトリクスを作ってbooleanで繋ぐ解法がなぜ問題を解決しないのか
- 13状態と15イベントを1つの遷移テーブルに統合して、UI優先表示(モーダル vs オーバーレイ)を型で閉じ込める方法
- Go標準ライブラリだけで書けるrepository / service / handlerのinterface分離設計
- TypeScriptの exhaustive check で状態の追加漏れをコンパイル時に捕捉する書き方
- GoとReactの両方を `bin/quality` とpre-commitに束ねて品質検証を自動化する方法

## スタック

| 層 | 技術 | 依存 |
| --- | --- | --- |
| バックエンド | Go 1.26 | 標準ライブラリのみ (`net/http`, `encoding/json`, `log/slog`, `sync`) |
| フロントエンド | React 19 + Vite + TypeScript | `react`, `react-dom` のみ (state管理ライブラリ不使用) |
| スタイリング | plain CSS | なし |
| 品質検証 (Go) | `gofmt`, `go vet`, `staticcheck`, `golangci-lint`, `go test -count=3 -race`, `go build` | - |
| 品質検証 (React) | `prettier`, `eslint`, `tsc --noEmit`, `vitest`, `vite build` | - |
| パッケージマネージャ | `pnpm` | - |

## 状態機械の概要

13状態を1つの遷移テーブルで管理し、状態ごとにUIサーフェス(none / modal / overlay / error / result)が1対1で決まります。同じ瞬間にモーダルとオーバーレイの両方が表示されることは構造上ありえません。

```
draft → uploading → parsing → validating → ready → importing → completed
                │         │          │         │         │
                ├→ upload_failed    │         ├→ cancelled
                ├→ parse_failed     │         └→ failed
                └→ validation_failed
```

詳細な遷移マトリクスは記事本文を参照してください。

## ディレクトリ構成

```
go-react-state-machine-import/
├── backend/                          # Go バックエンド
│   ├── cmd/server/                  # エントリポイント
│   ├── internal/statemachine/       # 汎用FSMコア
│   ├── internal/importjob/          # ドメイン (Repository / Service / Clock / IDSource)
│   ├── internal/httpapi/            # REST ハンドラ
│   ├── go.mod
│   └── .golangci.yml
├── frontend/                         # React フロントエンド
│   ├── src/state/                   # 遷移テーブル (TS)
│   ├── src/api/                     # fetch クライアント
│   ├── src/components/              # ImportJobWorkbench
│   ├── src/test/                    # vitest セットアップ
│   └── package.json
├── bin/
│   ├── quality                      # 両方の品質検証を一括実行
│   └── setup_hooks                  # pre-commit フックを有効化
└── .githooks/pre-commit             # git commit 前に bin/quality を走らせる
```

## セットアップ

### 必要なもの

- Go 1.22 以上 (1.26 で動作確認)
- Node.js 20 以上
- `pnpm` (`npm i -g pnpm`)
- `staticcheck` (`go install honnef.co/go/tools/cmd/staticcheck@latest`)
- `golangci-lint` (https://golangci-lint.run/usage/install/)

### インストールと初回ビルド

```bash
git clone https://github.com/okamyuji/go-react-state-machine-import.git
cd go-react-state-machine-import

# フロントエンドの依存をインストール
cd frontend && pnpm install && cd ..

# pre-commit フックを有効化
bin/setup_hooks

# 品質検証が一括で通ることを確認
bin/quality
```

## 起動

バックエンドとフロントエンドを別ターミナルで起動してください。

### バックエンド

```bash
cd backend
go run ./cmd/server
# → http://localhost:8080 でAPIが起動
```

### フロントエンド

```bash
cd frontend
pnpm run dev
# → http://localhost:5173 でUIが開く
```

ブラウザで <http://localhost:5173> を開くと、ImportJob状態遷移ワークベンチが表示されます。「作成」でジョブを作り、遷移ボタンを順に押していくと、各状態に対応するUIサーフェス(モーダル、オーバーレイ、エラー、結果表示)が1つずつ排他的に描画されるのを確認できます。

## 開発

### 品質検証の個別実行

```bash
# Go
(cd backend && gofmt -l .)
(cd backend && go vet ./...)
(cd backend && staticcheck ./...)
(cd backend && golangci-lint run ./...)
(cd backend && go test -count=3 -race ./...)
(cd backend && go build ./...)

# React
(cd frontend && pnpm run format:check)
(cd frontend && pnpm run lint)
(cd frontend && pnpm run typecheck)
(cd frontend && pnpm test)
(cd frontend && pnpm run build)
```

### pre-commit フックの動作確認

`bin/setup_hooks` を実行済みであれば、`git commit` 前に自動で `bin/quality` が走ります。どれか1つでも失敗するとコミットが中断されます。

### 主要なAPIエンドポイント

| Method | Path | 説明 |
| --- | --- | --- |
| `GET` | `/api/healthz` | ヘルスチェック |
| `GET` | `/api/import_jobs` | ジョブ一覧 |
| `POST` | `/api/import_jobs` | ジョブ作成 (`{"filename": "...", "rows": 100}`) |
| `GET` | `/api/import_jobs/{id}` | ジョブ取得 |
| `POST` | `/api/import_jobs/{id}/events` | 状態遷移イベントを適用 (`{"event": "upload"}`) |

無効な遷移は `409 Conflict` を返します。

## テスト戦略

- **Go**: 汎用 `statemachine`、ドメイン `importjob`、HTTP `httpapi` を個別にテスト。`TestViewForIsExclusive` で13状態すべての `state → view` 対応を網羅
- **React**: 遷移テーブルの到達可能性と一意性を `vitest` で担保。`ImportJobWorkbench` のコンポーネントテストで `role="dialog"` と `role="status"` が同時に現れないことを検証

## ライセンス

MIT License

## 関連記事

- [Go + Reactで現場レベルの状態遷移を1つのテーブルに統合する](https://zenn.dev/okamyuji/articles/golang-react-state-machine-transition-table) ← 本リポジトリの解説記事
- [Reactのフラグ地獄を状態遷移テーブルで解消する](https://zenn.dev/okamyuji/articles/react-state-pattern-finite-state-machine)
- [Railsのフラグ地獄を状態遷移テーブルで解消する](https://zenn.dev/okamyuji/articles/rails-state-machine-transition-table)
