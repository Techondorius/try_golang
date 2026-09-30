# 開発ガイド

このリポジトリは、devbox で Go と gopls を管理する小さな Go プロジェクトです。
モジュール名は `try_golang`、エントリーポイントは `main.go` です。

## Go コマンドの実行

Go コマンドは必ず devbox 経由で実行してください。ホスト環境の `go` を直接実行しないでください。

- 実行: `devbox run -- go run .`
- ビルド: `devbox run -- go build ./...`
- テスト: `devbox run -- go test ./...`
- 静的チェック: `devbox run -- go vet ./...`
- 整形: `devbox run -- go fmt ./...`
- 依存関係の整理: `devbox run -- go mod tidy`

`devbox shell` 内で Go コマンドを実行することもできます。
`devbox run test` でもテストを実行できます。`devbox run check` は race 検出付きテスト、静的チェック、ビルドを実行します。

## 変更時の方針

- Go の標準的な書式に従い、変更した Go ファイルを整形してください。
- 変更内容に応じてテストやビルドを実行してください。
- ユーザーの既存の変更を保持してください。
