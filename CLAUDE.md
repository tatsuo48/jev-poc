# CLAUDE.md

TypeSafe AI の判定モデル jev(文章を生成せず Choice / Score / Noul の判定だけを返す)に 2048 を遊ばせる PoC。Go の CLI と、Cloudflare Workers の Web デモの 2 つから成る。

- 公開デモ: https://jev-2048.personal-3f0.workers.dev
- リポジトリ: https://github.com/tatsuo48/jev-poc (パブリック)
- 結果と考察は `README.md`、Web デモの仕組みと悪用対策は `web/README.md`、設計の経緯は `docs/superpowers/specs/`

## 構成

| 場所 | 内容 |
|---|---|
| `cmd/jev2048`, `internal/` | Go CLI。`watch`(観戦)と `bench`(同じシードで成績比較)。標準ライブラリのみ |
| `internal/player/` | `random` / `greedy` / `expectimax` / `jev-raw` / `jev-sim`。追加は `registry.go` に 1 行 + 実装ファイル |
| `web/src/shared/` | 2048 のルールと jev へのプロンプト(TypeScript)。ブラウザと Worker の両方が使う |
| `web/src/worker/` | `POST /api/move`, `GET /api/status`。`index.ts` と `budget.ts` は配線だけで、ロジックは依存を注入できるモジュールに置く |
| `web/src/client/`, `web/public/` | ブラウザ側。ゲームループと古典 AI はブラウザで動く |

## コマンド

```bash
# Go(リポジトリ直下)
gofmt -l . && go vet ./... && go test -race ./...
go build -o jev2048 ./cmd/jev2048
./jev2048 watch --player jev-sim --seed 42
./jev2048 bench --players random,greedy,expectimax,jev-raw,jev-sim --games 5 --out results.jsonl

# Web(web/ の中)
npm run typecheck && npm test && npm run build
npm run dev        # http://localhost:8787 。実際の jev API を呼ぶ
npm run deploy     # ユーザーが実行する
```

## 守ること

- **API キー**: CLI は環境変数 `TYPESAFE_API_KEY` か、カレントディレクトリの `.env` から読む。Web のローカル実行は `web/.dev.vars`、本番は `wrangler secret`。`.env` と `.dev.vars` の中身は読まない・表示しない・コミットしない。コマンドに渡すときは値を出さずにパイプする(例: `grep '^TYPESAFE_API_KEY=' .env | cut -d= -f2-`)
- **公開リポジトリ**: push する前に履歴にキーが無いことを確認する。キーはコード・テスト・ログ・エラーメッセージ・レスポンスのどこにも出さない(上流のエラー本文も返さない・ログに残さない)
- **テストは実 API を叩かない**: Go は `httptest` と偽の `Asker`、Web は偽の `fetch` を注入する。実 API を使うのは `internal/jev/integration_test.go`(キーがあるときだけ走る 1 本)と、手動の `watch` / `bench` / `npm run dev` だけ
- **失敗した jev の手をランダム手などで穴埋めしない**: CLI はゲームを中断して記録し、Web は再試行のあと停止する。成績が汚れるため
- **Go と TypeScript でルールとプロンプトを一致させる**: `internal/game` と `web/src/shared/game.ts`、`internal/player/jev.go` と `web/src/shared/prompt.ts`。プロンプトの文面は一字一句同じにする(ベンチ結果と同じ条件でデモを動かすため)。片方を変えたらもう片方も変える
- **jev は合法手だけを選択肢として渡す**。合法手が 1 つの手番は API を呼ばない
- **デプロイ、`wrangler login`、`wrangler secret put` はユーザーが実行する**。コマンドを提示して 1 つずつ案内する(ユーザーは Workers に詳しくない)
- 実 API を大量に呼ぶ前に、呼び出し回数と費用の見積もりを示して確認を取る。料金は入力 100 万トークンあたり $0.042(出力無料)で、5 プレイヤー × 5 ゲームのベンチが約 $0.05

## 知っておくと困らないこと

- jev の判定は非決定的。同じ盤面でも確率と confidence が揺れるので、jev 系のゲームは同じシードでも再現しない
- Go と Web では乱数が別物。同じシード番号でも盤面は一致しない
- Web の 2 つの tsconfig: `tsconfig.json`(ブラウザ・共有・テスト、DOM 型)と `tsconfig.worker.json`(Worker、`@cloudflare/workers-types`)。両者の型は衝突するので混ぜない。`cloudflare:workers` を import するファイルは vitest から import できない
- Workers 無料プランの制約が設計を決めている: 1 リクエスト内の `fetch` は 50 回まで(だからゲームはブラウザが進める)、CPU 10ms(だから `expectimax` はブラウザで動かす)、1 日 10 万リクエストはアカウント全体で共有
- Web デモの歯止め: IP ごとに 10 秒 150 回、全訪問者合計で 1 日 20,000 手(`web/wrangler.jsonc` の `DAILY_LIMIT`。`"0"` で jev 系を即停止)。レート制限が落ちたら通す、上限カウンタが落ちたら jev を呼ばず 503
- 初回デプロイは `npm run deploy` → `secret put` の順(逆だと Worker 作成の確認を対話で聞かれる)。キー未登録の間は 503 を返し、上限も消費しない
- `preview_urls` は意図して無効(古いバージョンを別 URL で触れる状態にしない)
- `web/public/_headers` の CSP は `default-src 'self'`。インラインの script / style / イベントハンドラを足すと動かなくなる
- ユーザーのシェルは fish。アプリの Run ボタンは実行ごとに環境変数が引き継がれないことがあるので、キーは `.env` 経由で渡す
- コミットメッセージの作法: `feat(web): ...` / `fix(cli): ...` / `docs: ...`

## 次にやると面白そうなこと

- confidence が低い手番だけ `expectimax` に任せる混成プレイヤー(confidence 0.5 以上の手はほぼ必ず最大得点の手だった)
- ゲーム数を増やして `jev-sim` と `greedy` の順位を確定させる(5 ゲームでは振れが大きい)
