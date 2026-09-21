# jev-2048 Web デモ 設計書

- 日付: 2026-09-21
- ステータス: 承認済み(チャットで合意)
- 前提: [jev-2048 設計書](2026-09-21-jev-2048-design.md) の CLI は完成済み。ここではその観戦機能をブラウザに移す

## 目的

不特定多数の訪問者が、ブラウザで「AI を選んでスタートを押し、2048 を 1 手ずつ打つ様子を観戦できる」ページを、Cloudflare Workers の無料プランで公開する。jev 系のプレイヤーでは、各手の確率分布と confidence も見せる。

## 制約

- Cloudflare の費用は 0 円(Workers Free プラン)に収める
  - 1 日 10 万リクエスト、1 リクエストあたり CPU 10ms(`fetch` の待ち時間は含まれない)、1 リクエスト内の `fetch` は 50 回まで
  - Durable Objects は SQLite バックエンドのみ利用可。1 日 10 万リクエスト、行の書き込み 1 日 10 万
- jev の利用料(入力 100 万トークンあたり $0.042)に上限を付ける。全訪問者の合計で **1 日 20,000 手**まで。最悪でも 1 日約 $0.8
- API キーはブラウザに渡さない。リポジトリ・ログ・レスポンスにも出さない
- 公開エンドポイントを「任意の内容で jev を呼べるプロキシ」にしない

## スコープ外

- 人間がプレイするモード、2 つの AI の同時対決、ベンチマーク機能
- Go の乱数列の再現。同じシード番号でも CLI とは別の盤面になる
- Turnstile などのボット対策。1 日の上限で費用は膨らまないので、荒らされてから足す
- ユーザー登録、スコアの保存、ランキング
- 独自ドメイン。`*.workers.dev` で公開する

## 方式

ゲームはブラウザが進める。Worker は jev への中継だけを行う。

```
ブラウザ(静的ページ)
  ├─ 盤面・ルール・描画・確率バー
  ├─ random / greedy / expectimax はブラウザ内で計算(Worker を呼ばない)
  └─ jev の手番だけ POST /api/move {player, board}
                        │
Worker
  ├─ 送信元と入力を検証し、プロンプトを盤面から自分で組み立てる
  ├─ IP ごとのレート制限、1 日の上限(Durable Object)
  ├─ シークレットのキーを付けて jev API を 1 回呼ぶ
  └─ {move, probabilities, confidence, remaining} を返す
```

ゲームのループを Worker の中で回さない理由: 1 ゲームで jev を 200〜400 回呼ぶため、無料プランの「1 リクエスト内の `fetch` 50 回」を超える。`expectimax` を Worker で動かさない理由: Go でも 1 手 4ms かかり、CPU 10ms の上限に近い。

## 構成

同じリポジトリの `web/` 以下。TypeScript、フレームワークなし。依存は開発用の `wrangler`、`typescript`、`esbuild`、`vitest` だけ。

```
web/
  package.json
  tsconfig.json
  wrangler.jsonc
  public/index.html          静的ページ(ビルドした client.js を読み込む)
  public/style.css
  src/shared/game.ts         盤面、スライド、合法手、特徴量、シード付き乱数、Game
  src/shared/prompt.ts       instructions、jev-raw / jev-sim の state と criteria
  src/client/players.ts      random / greedy / expectimax、jev(/api/move を呼ぶ)
  src/client/loop.ts         ゲームループ(開始、停止、速さ、再試行)
  src/client/view.ts         DOM の描画(盤面、確率バー、ステータス)
  src/client/main.ts         画面の部品とループをつなぐ
  src/worker/index.ts        ルーティングと /api/move、/api/status
  src/worker/validate.ts     リクエストの検証
  src/worker/jev.ts          jev API の呼び出し
  src/worker/budget.ts       1 日の上限を数える Durable Object
  test/                      vitest
```

### shared/game.ts

Go の `internal/game` と同じ規則。

- `Board` は 4×4 の数値配列(空きは 0)、`Move` は `"up" | "down" | "left" | "right"`、`ALL_MOVES` はこの順
- `slide(board, move)` は `{board, gained, merges, moved}` を返す。1 回の移動で同じタイルは 1 度しかマージしない(`[2,2,2,2]` → `[4,4,0,0]`)
- `legalMoves`, `emptyCells`, `maxTile`, `maxInCorner`
- 乱数は mulberry32(32bit シード)。`spawn` は空きマスに 90% で 2、10% で 4
- `Game`: `board`, `score`, `moves`、`step(move)`, `over()`。同じシードと同じ手の列なら同じ盤面列になる
- Go のテーブルテストと同じケースで検証する

### shared/prompt.ts

`instructions`、方向の説明、`jev-raw` の `state`(`{board}`)、`jev-sim` の `state`(`{board, candidates}`。候補ごとに `board`, `gained`, `empty_cells`, `merges`, `max_tile`, `max_in_corner`)と `criteria` は、Go の `internal/player/jev.go` と同じ文面・同じ形にする。`criteria` には合法手だけを入れる。

### Worker

`POST /api/move`。処理はこの順で、どこかで失敗したらそこで応答を返す。

| 順 | 処理 | 失敗時 |
|---|---|---|
| 1 | メソッドが POST、`Content-Type` が JSON | 405 / 415 |
| 2 | `Origin` ヘッダがリクエストの URL と同じオリジン | 403 `forbidden_origin` |
| 3 | 本文が 2KB 以下(413 `payload_too_large`)。`player` が `jev-raw` か `jev-sim`。`board` が 4×4 で、各マスが 0 か 2〜131072 の 2 の累乗。合法手が 2 つ以上 | 400 `invalid_request` |
| 4 | IP(`CF-Connecting-IP`)ごとのレート制限: 10 秒あたり 150 回(当初は 50 回。1 ゲームが 10 秒に 30〜40 回呼ぶため、1 つの IP を複数人が共有すると厳しすぎた) | 429 `rate_limited` |
| 5 | 1 日の上限: Durable Object で 1 手ぶん確保する | 429 `daily_budget_exhausted` |
| 6 | jev API を 1 回呼ぶ(タイムアウト 10 秒)。Worker 内では再試行しない | 502 `upstream_error` |
| 7 | 応答の `choice` が合法手であることを確認する | 502 `upstream_error` |

成功時の応答:

```json
{"move": "down", "probabilities": {"down": 0.56, "left": 0.39, "up": 0.02, "right": 0.03}, "confidence": 0.42, "remaining": 18230}
```

エラー時の応答は `{"error": "<コード>"}` だけ。上流のエラー本文やステータスの詳細は返さない(キーや内部情報が混ざる経路を作らない)。`console.error` にも上流の本文は出さず、ステータスコードだけを出す。

- 合法手が 1 つの手番はブラウザ側で処理し、API を呼ばない(CLI と同じ)。Worker は念のため順 3 で弾く
- 上流が失敗しても、順 5 で確保した 1 手ぶんは戻さない(実装を単純に保つ。上限が少し早く減るだけで、費用が増える方向には働かない)
- Rate Limiting binding が無料プランで使えなかった場合は、順 4 を外す。1 日の上限があるので費用の上限は変わらない

`GET /api/status` は `{"remaining": <数>, "limit": 20000}` を返す。上限は消費しない。

それ以外のパスは静的ファイル(Workers の Static Assets)。

### worker/budget.ts

SQLite バックエンドの Durable Object を 1 つだけ(名前 `global`)使う。

- `take()`: UTC の日付をキーに、その日の消費数が 20,000 未満なら 1 増やして `{ok: true, remaining}` を返す。達していれば `{ok: false, remaining: 0}`
- `peek()`: 消費せずに残りを返す
- 保存するのは「日付と消費数」の 1 件だけ。日付が変わった最初の `take()` で上書きされる
- 上限値は `wrangler.jsonc` の変数 `DAILY_LIMIT`(既定 20000)で変えられる

### クライアント

- 操作: プレイヤー(`jev-sim` / `jev-raw` / `expectimax` / `greedy` / `random`、既定は `jev-sim`)、シード(数値、既定 42)、速さ(ゆっくり 500ms / ふつう 150ms / 速い 0ms)、スタート / ストップ
- 表示: 盤面(タイルの値で色分け)、score、moves、直前の手、1 手のレイテンシ。jev 系では合法手ごとの確率バーと confidence、「本日の jev 残り N 手」。古典 AI ではバーと confidence を出さない
- ページ下部に「これは何?」: jev の説明、`jev-raw` と `jev-sim` の違い、CLI のベンチ結果の表、GitHub へのリンク
- `expectimax` は深さ 3。メインスレッドで 1 手ずつ計算し、手と手の間で描画に制御を返す
- jev の手番で 429 `rate_limited` か 502 が返ったら、1 秒、2 秒、4 秒と間隔を空けて最大 3 回まで再試行する。それでも失敗したらゲームを止めて「通信に失敗しました。再開する」ボタンを出す。ランダム手での穴埋めはしない
- `daily_budget_exhausted` のときはゲームを止め、「本日の jev は終了しました。古典 AI は引き続き遊べます」と表示する
- スマホの幅でも崩れないレイアウトにする

## 設定とシークレット

- `TYPESAFE_API_KEY`: `wrangler secret put` で登録する。ローカル開発では `web/.dev.vars`(git 管理外)に置く
- `JEV_ENDPOINT`: 変数。既定は `https://api.typesafe.ai/v1/systemone`
- `DAILY_LIMIT`: 変数。既定は `20000`

## テスト

vitest。実 API は叩かない。

- `shared/game`: `slide` のテーブルテスト(Go と同じケース)、`legalMoves`、特徴量、同一シードでの決定性、ゲーム終了判定、`spawn`
- `shared/prompt`: `jev-raw` の `state` が盤面だけであること、`jev-sim` の候補が合法手の数だけあり特徴量が正しいこと、`criteria` が合法手だけであること
- `client/players`: `greedy` の同点処理、`expectimax` が `random` より合計スコアで上回ること
- `worker/validate`: 正常な盤面、形の違う盤面、2 の累乗でない値、合法手が 1 つ以下の盤面
- `worker/index`: jev への `fetch`、上限カウンタ、レート制限を偽物に差し替えて、表の各行(405 / 415 / 413 / 403 / 400 / 429 ×2 / 502 ×2 / 200)と、エラー応答に上流の本文が含まれないこと、jev へのリクエストに `Authorization` が付きブラウザへの応答には付かないことを確認する
- `worker/budget`: 上限ちょうどまで `take()` が成功し、その次が失敗すること、日付が変わるとリセットされること(時刻は注入する)

## デプロイ

1. `cd web && npm install`
2. `npx wrangler login`(ブラウザで Cloudflare の認可)
3. `npx wrangler secret put TYPESAFE_API_KEY`
4. `npm run deploy`(クライアントのビルド → `wrangler deploy`)
5. `https://jev-2048.<サブドメイン>.workers.dev` で動作を確認する

2〜4 はユーザーのアカウントで外部に公開する操作なので、コマンドを提示してユーザーが実行する。

## 実装の順序

1. `web/` の足場(package.json、tsconfig、wrangler.jsonc、vitest が動くところまで)
2. `shared/game`
3. `shared/prompt`
4. `worker`(validate、jev、budget、index)
5. `client`(players、loop、view、main、HTML / CSS)
6. `wrangler dev` でローカル確認 → デプロイ → README に URL と仕組みを追記
