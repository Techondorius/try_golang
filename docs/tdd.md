# TDDの記録

[t-wadaによるTDDの定義の翻訳](https://t-wada.hatenablog.jp/entry/canon-tdd-by-kent-beck) を参考に、期待する振る舞いの洗い出し、Red → Green、Green後のRefactorを行っています。ドメイン → ユースケース → アダプター → HTTPの順に実装しました。初期の増分ではパッケージ単位で複数のテストを先に書いており、「一つだけ選んで進める」という手順を厳密に守れていない箇所があります。末尾スラッシュの増分は、テストリストから一つ選び、単一の失敗を確認してから変更しています。

## テストリスト

- [x] Emailを正規化し、不正な形式と長すぎる入力を拒否する
- [x] パスワードの12〜72バイト制約を守る
- [x] UserのID・Email・パスワードハッシュは必須
- [x] Taskはタイトルを正規化し、所有者と作成日時を保持する
- [x] タイトル不正時にTaskを変更しない
- [x] 完了したTaskの改名・再完了を拒否する
- [x] 登録したユーザーがログインでき、同じメールは二重登録できない
- [x] ユーザー未存在でもパスワード照合を実行する
- [x] 生のパスワードとBearerトークンを保存しない
- [x] セッションは期限の境界で失効する
- [x] ログアウトは現在のセッションだけを失効させる
- [x] ユーザーが消えたセッションからは認証できない
- [x] 他ユーザーのタスクは取得・改名・完了・削除できず、一覧にも出ない
- [x] 同時登録と同時完了はそれぞれ一度だけ成功する
- [x] 保存層から取得したコピーの変更は、保存済み集約に影響しない
- [x] 更新コールバックが失敗したら保存しない
- [x] 期限切れセッションを新規ログイン時に掃除する
- [x] HTTPで登録 → ログイン → タスク操作 → ログアウトが動く
- [x] HTTP入力の未知フィールド・不正JSON・複数値・容量超過を拒否する
- [x] 認証エラー・未存在・競合・内部障害をJSONと適切なステータスに変換する
- [x] 認証APIの回数を接続元IPごとに制限し、時間経過後に再開する
- [x] 転送元ヘッダーの偽装で回数制限を回避できない
- [x] 同時リクエストでも回数制限を越えて受け付けない
- [x] 暗号処理や保存が失敗したら、登録を保存せずトークンを発行しない
- [x] panicを500へ変換し、Bearerトークンやpanicの内容をログへ出さない
- [x] 設定の不正なポートやセッション期限で起動を拒否する
- [x] ドメインやユースケースから外側の層への依存を禁止する
- [x] 未定義の末尾スラッシュURLも、JSONの404として応答する

## 実際に確認したRed → Green

以下は今回の実装時に実行したテストの記録です。初回の各パッケージでは実装ファイルやAPIがまだないため、Redはコンパイル失敗から始まりました。回数制限では既に動くAPIにテストを追加して、期待する振る舞いとの差を確認しています。

| 増分 | Redの観測 | Greenの変更 |
| --- | --- | --- |
| `domain/account`, `domain/task` | `no non-test Go files` | 値検証とTaskのライフサイクルを実装 |
| `application` | `no non-test Go files` | 登録、ログイン、認証、失効、所有者チェックを実装 |
| `infrastructure` | `no non-test Go files` | ポートを満たすメモリ保存、bcrypt、乱数生成を実装 |
| `transport/httpapi` | `no non-test Go files` | Ginルート、認証、DTO、HTTPエラー、厳密なJSON入力を実装 |
| 認証API回数制限 | `status 401, want 429`、`accepted 20, limited 0` | mutex付き固定窓と、信頼しない転送元の設定で制限を実装 |
| `config` | `no non-test Go files` | デフォルト値、待受アドレスと期限の検証を実装 |
| Userの生成条件 | `assignment mismatch: 2 variables but account.NewUser returns 1 value` | 不正な集約を生成せずエラーを返すAPIに変更 |
| 未定義URLの末尾スラッシュ | `status 301, want 404` | Ginの自動リダイレクトを無効にし、既存のJSON 404処理に統一 |

最初の環境では既存のGoキャッシュにより標準パッケージの解決に失敗しました。これは機能のRedとは区別し、別のビルドキャッシュで再実行してGreenを確認しました。その後 `devbox.json` にプロジェクト専用の `GOCACHE` を設定しています。

初期のコマンド例:

```sh
GOCACHE=/private/tmp/try-golang-build-cache devbox run -- go test ./internal/domain/...
GOCACHE=/private/tmp/try-golang-build-cache devbox run -- go test ./internal/application/...
GOCACHE=/private/tmp/try-golang-build-cache devbox run -- go test ./internal/infrastructure/...
GOCACHE=/private/tmp/try-golang-build-cache devbox run -- go test ./internal/transport/httpapi/...
GOCACHE=/private/tmp/try-golang-build-cache devbox run -- go test ./internal/transport/httpapi -run 'TestAuthRateLimit|TestConcurrentRateLimit'
```

## Refactor

HTTPの動作テストを通した後、ルート登録・ハンドラー・ミドルウェアを別ファイルに分けました。Taskのタイトル検証はTitle値オブジェクトへ抽出しました。依存の向き、公開APIの振る舞い、状態遷移を変えずに全テストを再実行しています。

エラー伝播や複数セッションの独立性については、既存動作を固定する回帰テストも追加しました。テスト全てが新機能のRedから始まったという記録ではありません。

末尾スラッシュの改善では `TestTrailingSlashErrorContract` を一つ追加し、301応答によるRedを確認した後、ルーター設定を一行変更してGreenにしました。追加の構造変更は不要だったため、この増分では新しい抽象化を導入していません。

## テストの境界

- ドメイン: 入力と状態遷移を検証し、Ginや保存層を使わない
- ユースケース: 保存・ハッシュ・時刻・IDをFakeへ差し替え、認証・所有者チェック・失敗時の振る舞いを検証
- インフラ: 本物のメモリ保存とbcryptを使い、同時実行・値の隔離・ハッシュ照合を検証
- HTTP: `httptest` から本物のGin・ユースケース・メモリ保存・bcryptを通し、応答とヘッダーを検証
- アーキテクチャ: 本番ソースのimportを検査し、依存方向を検証

ユースケースのテストでは時刻を固定し、期限切れまで実時間で待ちません。並行処理のテストは `sync.WaitGroup` と `atomic` を使います。HTTPテストのbcryptは最低costを使って高速化し、本番の起動コードはcost 12を使います。

## 検証コマンド

```sh
devbox run -- go fmt ./...
devbox run -- go mod tidy
devbox run -- go test -race ./...
devbox run -- go vet ./...
devbox run -- go build ./...
```

実サーバーでも2ユーザーを登録して、本人のタスク操作、他ユーザーからの404、ログアウト後の401を確認しました。SIGTERMを受けたプロセスの正常終了も確認しています。GitHub Actionsの設定は同じdevboxコマンドを実行します。CI実行結果はGitHubへpushした後に確認してください。

今後の追加機能も、まずこのテストリストに振る舞いを追加し、一つの失敗するテストから始めてください。例として永続保存を追加する場合、再起動後に同じユーザー・タスクを取得できるテストを書き、Repositoryの契約を守るアダプターを実装します。
