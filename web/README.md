# jev plays 2048 — Web デモ

訪問者が AI を選んでスタートを押すと、2048 を 1 手ずつ打つ様子を観戦できるページ。Cloudflare Workers の無料プランで動く。

公開先: https://jev-2048.personal-3f0.workers.dev

## 仕組み

ゲームはブラウザが進め、Worker は jev への中継だけを行う。

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

こうしている理由は無料プランの上限にある。1 リクエストの中で外部に `fetch` できるのは 50 回までで、1 ゲームは jev を 200〜400 回呼ぶので、Worker の中でゲームを回すことはできない。CPU 時間も 1 リクエスト 10ms までなので、1 手に数 ms〜数十 ms かかる `expectimax` はブラウザで動かす。`fetch` の待ち時間は CPU 時間に数えられないので、jev の応答を待つ約 0.2 秒は問題にならない。

| ディレクトリ | 内容 |
|---|---|
| `src/shared/` | 2048 のルールと、jev に渡すプロンプト。ブラウザと Worker の両方が使う。Go の CLI と同じ規則・同じ文面 |
| `src/worker/` | `/api/move` と `/api/status`。`index.ts` と `budget.ts` は Workers ランタイムへの配線だけで、ロジックは依存を注入できるモジュールに置いてテストしている |
| `src/client/` | プレイヤー、ゲームループ、描画 |
| `public/` | 静的ファイル。`client.js` はビルドで生成する(git 管理外) |

乱数は Go 版と別物なので、同じシード番号でも CLI とは別の盤面になる。

## 悪用への備え

このページは誰でも開けて、jev を 1 回呼ぶたびに API の利用料がかかる。

| 備え | 内容 |
|---|---|
| キーの隔離 | API キーは Worker のシークレット。ブラウザにも応答にもログにも出さない。上流のエラー本文は返さず、ログにも残さない |
| プロキシにしない | Worker が受け取るのはプレイヤー名と盤面だけ。盤面は 4×4・各マスが 0 か 2 の累乗であることを検証し、プロンプトは Worker が自分で組み立てる。任意の文章を jev に送る手段にはならない |
| 送信元の確認 | `Origin` がこのサイトと一致しないリクエストは拒否する(ブラウザ経由の他サイトからの利用を防ぐ。`curl` などは偽装できるので、費用の歯止めは下の 2 つが担う) |
| 本文の大きさ | 2KB を超えた時点で読み込みを打ち切る |
| IP ごとのレート制限 | 10 秒あたり 150 回。1 ゲームは 10 秒に 30〜40 回呼ぶので、同じ IP から同時に遊べるのは 3〜4 ゲーム(オフィスや会場の回線など、1 つの IP を複数人が共有する場合を見込んだ値)。カウントは拠点ごとのおおよその値。役目は「1 つの IP が共有の 1 日上限を使い切る速さ」を抑えることで、この値だと最短で約 22 分かかる。費用の歯止めは下の 1 日の上限が担う |
| 1 日の上限 | 全訪問者の合計で 1 日 20,000 手(UTC)。Durable Object で正確に数える。超えたら jev 系は翌日まで休みになり、古典 AI は引き続き遊べる。費用は最悪でも 1 日約 $0.8 |

レート制限の仕組みが落ちているときは制限なしで通す(1 日の上限が残っているので費用は膨らまない)。上限カウンタが落ちているときは jev を呼ばずに 503 を返す。

## ローカルで動かす

```bash
npm install
```

`web/.dev.vars`(git 管理外)に API キーを置く。

```
TYPESAFE_API_KEY=apikey_...
```

```bash
npm run dev
```

`http://localhost:8787` で開く。ローカルでも実際の jev API を呼ぶ。

```bash
npm test
```

```bash
npm run typecheck
```

テストは実 API を叩かない。

## デプロイ

Cloudflare のアカウント(無料プランでよい)が要る。

```bash
npx wrangler login
```

```bash
npx wrangler secret put TYPESAFE_API_KEY
```

```bash
npm run deploy
```

`https://jev-2048.<あなたのサブドメイン>.workers.dev` で公開される。初回は先に `npm run deploy` をしてから `secret put` をするとよい(Worker がまだ無い状態で `secret put` をすると、作成の確認を対話で聞かれる)。キーが未登録の間、Worker は jev を呼ばずに 503 を返し、1 日の上限も消費しない。

- 1 日の上限は `wrangler.jsonc` の `DAILY_LIMIT` で変える
- Rate Limiting binding がプランの都合で拒否された場合は、`wrangler.jsonc` の `ratelimits` を消して再デプロイする。コードは binding が無ければレート制限を飛ばす
- 公開をやめるときは `npx wrangler delete`
