# 分類（decision-log）

フロントマターの `category` に 1 つだけ付ける。迷ったら、変更がいちばん守ろうとしているものを選ぶ（例：個人情報をログに出すのをやめた → `privacy`、ログの出し方を変えた → `release`）。中身の列は、索引の各見出しの直下にもそのまま出る。

| `category` | 見出し | 中身 |
|---|---|---|
| `architecture` | 設計・構成 | 分割と統合、依存の向き、採用する技術、基盤の移行 |
| `data` | データの意味・持ち方 | スキーマ、値の意味と単位、保存先、保持期間（個人情報は privacy） |
| `security` | 認証・権限・セキュリティ | 認証方式、権限の境界、秘密情報、署名と検証、暗号化 |
| `privacy` | 個人情報 | 個人情報を集めるか、どこに出す・送る・残すか（ログ・送信先・保持期間）、マスキング |
| `compatibility` | 互換性・外部との約束 | 公開 API、形式、バージョン、廃止 |
| `reliability` | 性能・信頼性 | 負荷や障害のときの振る舞い。タイムアウト、リトライ、上限、キャッシュ、縮退 |
| `release` | リリース・運用 | デプロイの手順、フラグ、環境、監視・アラートと通知先、ログ・トレースの出し方 |
| `engineering` | 開発の決まり | CI の必須チェック、テストの方針、lint・コードの規約、ブランチの運用 |
| `cost` | コスト | インスタンスやプランの大きさ、サンプリングの割合、コストのための削減や上限 |
| `product` | プロダクトの方針 | 仕様の線引き、やらないと決めたこと |
| `other` | その他 | どれにも当たらないもの。多ければ分類を見直す |

## 英語で書くとき

索引の見出しと中身は次のとおりになる（[writing.md](writing.md#英語で書くとき)）。

| `category` | Heading | About |
|---|---|---|
| `architecture` | Architecture | Splitting and merging, direction of dependencies, adopted technology, platform migrations |
| `data` | Data meaning and storage | Schemas, meaning and units of values, where data is stored, retention (personal data is privacy) |
| `security` | Authentication, permissions and security | Authentication methods, permission boundaries, secrets, signing and verification, encryption |
| `privacy` | Personal data | Whether personal data is collected, where it is shown, sent or kept (logs, destinations, retention), masking |
| `compatibility` | Compatibility and external contracts | Public APIs, formats, versions, deprecation |
| `reliability` | Performance and reliability | Behavior under load and failure: timeouts, retries, limits, caches, degradation |
| `release` | Release and operations | Deployment steps, flags, environments, monitoring, alerts and who gets them, logging and tracing |
| `engineering` | Engineering rules | Required CI checks, testing policy, lint and code conventions, branch workflow |
| `cost` | Cost | Instance and plan sizes, sampling rates, cuts and limits made for cost |
| `product` | Product policy | Where the spec draws the line, things decided not to do |
| `other` | Other | Anything that fits none of the above; if there are many, revisit the categories |
