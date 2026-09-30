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

## 初学者向けのコメント

- 本番コードの関数・構造体・interfaceには、日本語で責務と設計の理由を記載してください。
- 公開APIのコメントは対象の識別子名から始め、`go doc` で読める形式にしてください。
- ユースケースとドメインの責務の違い、依存の方向、DIで差し替えられる部分を説明してください。
- interfaceのメソッドは引数名を明示し、エラー・一意性・原子的な更新などの契約をコメントに記載してください。
- コメントは実装と一致させ、ゼロ値・nil・失敗時の扱いなど、実装が保証する範囲を説明してください。

## ユースケースの命名

- `internal/application` のユースケースを表す公開関数・メソッド名には、末尾に `UseCase` を付けてください。
- 例: `RegisterUseCase`、`LoginUseCase`、`CreateUseCase`、`CompleteUseCase`。
- ユースケースの構築関数も `NewAuthUseCase`、`NewTasksUseCase` のようにそろえてください。
- 集約や値オブジェクトの操作、保存ポートのメソッド、非公開の補助関数は対象外です。例: `Task.Rename`、`ITasksRepository.UpdateTask`、`change`。
- 改名時は呼び出し元・テスト・コメント・READMEも更新し、古い名称を参照する説明を残さないでください。

## Repositoryとポートの配置・命名

- 業務上の集約のRepository interfaceは、その集約と同じドメインパッケージに置いてください。
- このリポジトリではinterface名に `I` を付け、集約の保存契約は `IUsersRepository`、`ITasksRepository` のように命名してください。
- 具体的な保存実装は `internal/infrastructure` に置き、ユースケースへコンストラクターで注入してください。
- 認証セッションの保存は `application.ISessionsRepository`、パスワード処理は `application.IPasswordHasher` として、認証処理の技術的なポートをapplicationに置いてください。全てのinterfaceをRepositoryとは呼びません。
- 保存結果の共通エラーは `internal/domain/errors.go` に定義し、ドメインからapplicationやinfrastructureへの依存を作らないでください。
