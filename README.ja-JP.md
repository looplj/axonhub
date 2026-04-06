<div align="center">

# AxonHub - オールインワンAI開発プラットフォーム
### あらゆるSDKを使用。あらゆるモデルにアクセス。コード変更ゼロ。

<a href="https://trendshift.io/repositories/16225" target="_blank"><img src="https://trendshift.io/api/badge/repositories/16225" alt="looplj%2Faxonhub | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>

</div>

<div align="center">

[![Test Status](https://github.com/looplj/axonhub/actions/workflows/test.yml/badge.svg)](https://github.com/looplj/axonhub/actions/workflows/test.yml)
[![Lint Status](https://github.com/looplj/axonhub/actions/workflows/lint.yml/badge.svg)](https://github.com/looplj/axonhub/actions/workflows/lint.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/looplj/axonhub?logo=go&logoColor=white)](https://golang.org/)
[![Docker Ready](https://img.shields.io/badge/docker-ready-2496ED?logo=docker&logoColor=white)](https://docker.com)

[English](README.md) | [中文](README.zh-CN.md) | [日本語](README.ja-JP.md)

</div>

---

> 注意事項
>
> 1. このプロジェクトは個人によって運営されています。作者は、使用に起因するリスクに対していかなる保証も責任も負いません。慎重にご検討ください。
> 2. このプロジェクトのコアスコープには2api（サブスクリプションからAPIへの変換）は含まれていません。必要な場合は、2apiに特化した他のオープンソースプロジェクトをご検討ください。

---

## 📖 プロジェクト紹介

### オールインワンAI開発プラットフォーム

**AxonHubは、コードを一行も変更することなくモデルプロバイダーを切り替えられるAIゲートウェイです。**

OpenAI SDK、Anthropic SDK、またはその他のAI SDKを使用している場合でも、AxonHubはリクエストを透過的に変換し、サポートされているあらゆるモデルプロバイダーで動作させます。リファクタリングもSDKの入れ替えも不要 - 設定を変更するだけで完了です。

**解決する課題：**
- 🔒 **ベンダーロックイン** - GPT-4からClaudeやGeminiへ瞬時に切り替え
- 🔧 **統合の複雑さ** - 10以上のプロバイダーに対して単一のAPIフォーマット
- 📊 **オブザーバビリティの不足** - すぐに使えるリクエストトレーシング
- 💸 **コスト管理** - リアルタイムの使用量追跡と予算管理

<div align="center">
  <img src="docs/axonhub-architecture-light.svg" alt="AxonHub Architecture" width="700"/>
</div>

### コア機能

| 機能 | 提供する価値 |
|---------|-------------|
| 🔄 [**あらゆるSDK → あらゆるモデル**](docs/en/api-reference/openai-api.md) | OpenAI SDKでClaudeを呼び出したり、Anthropic SDKでGPTを呼び出したり。コード変更不要。 |
| 🔍 [**完全なリクエストトレーシング**](docs/en/guides/tracing.md) | スレッド対応のオブザーバビリティで完全なリクエストタイムラインを提供。デバッグを高速化。 |
| 🔐 [**エンタープライズRBAC**](docs/en/guides/permissions.md) | きめ細かなアクセス制御、使用量クォータ、データ分離。 |
| ⚡ [**スマートロードバランシング**](docs/en/guides/load-balance.md) | 100ms未満の自動フェイルオーバー。常に最も正常なチャネルにルーティング。 |
| 💰 [**リアルタイムコスト追跡**](docs/en/guides/cost-tracking.md) | リクエストごとのコスト内訳。入力、出力、キャッシュトークン - すべて追跡。 |

---

## 📚 ドキュメント

詳細な技術ドキュメント、APIリファレンス、アーキテクチャ設計などについては、以下をご覧ください
- [![DeepWiki](https://img.shields.io/badge/DeepWiki-looplj%2Faxonhub-blue.svg?logo=data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACwAAAAyCAYAAAAnWDnqAAAAAXNSR0IArs4c6QAAA05JREFUaEPtmUtyEzEQhtWTQyQLHNak2AB7ZnyXZMEjXMGeK/AIi+QuHrMnbChYY7MIh8g01fJoopFb0uhhEqqcbWTp06/uv1saEDv4O3n3dV60RfP947Mm9/SQc0ICFQgzfc4CYZoTPAswgSJCCUJUnAAoRHOAUOcATwbmVLWdGoH//PB8mnKqScAhsD0kYP3j/Yt5LPQe2KvcXmGvRHcDnpxfL2zOYJ1mFwrryWTz0advv1Ut4CJgf5uhDuDj5eUcAUoahrdY/56ebRWeraTjMt/00Sh3UDtjgHtQNHwcRGOC98BJEAEymycmYcWwOprTgcB6VZ5JK5TAJ+fXGLBm3FDAmn6oPPjR4rKCAoJCal2eAiQp2x0vxTPB3ALO2CRkwmDy5WohzBDwSEFKRwPbknEggCPB/imwrycgxX2NzoMCHhPkDwqYMr9tRcP5qNrMZHkVnOjRMWwLCcr8ohBVb1OMjxLwGCvjTikrsBOiA6fNyCrm8V1rP93iVPpwaE+gO0SsWmPiXB+jikdf6SizrT5qKasx5j8ABbHpFTx+vFXp9EnYQmLx02h1QTTrl6eDqxLnGjporxl3NL3agEvXdT0WmEost648sQOYAeJS9Q7bfUVoMGnjo4AZdUMQku50McDcMWcBPvr0SzbTAFDfvJqwLzgxwATnCgnp4wDl6Aa+Ax283gghmj+vj7feE2KBBRMW3FzOpLOADl0Isb5587h/U4gGvkt5v60Z1VLG8BhYjbzRwyQZemwAd6cCR5/XFWLYZRIMpX39AR0tjaGGiGzLVyhse5C9RKC6ai42ppWPKiBagOvaYk8lO7DajerabOZP46Lby5wKjw1HCRx7p9sVMOWGzb/vA1hwiWc6jm3MvQDTogQkiqIhJV0nBQBTU+3okKCFDy9WwferkHjtxib7t3xIUQtHxnIwtx4mpg26/HfwVNVDb4oI9RHmx5WGelRVlrtiw43zboCLaxv46AZeB3IlTkwouebTr1y2NjSpHz68WNFjHvupy3q8TFn3Hos2IAk4Ju5dCo8B3wP7VPr/FGaKiG+T+v+TQqIrOqMTL1VdWV1DdmcbO8KXBz6esmYWYKPwDL5b5FA1a0hwapHiom0r/cKaoqr+27/XcrS5UwSMbQAAAABJRU5ErkJggg==)](https://deepwiki.com/looplj/axonhub)
- [![zread](https://img.shields.io/badge/Ask_Zread-_.svg?style=flat&color=00b0aa&labelColor=000000&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB3aWR0aD0iMTYiIGhlaWdodD0iMTYiIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuOTYxNTYgMS42MDAxSDIuMjQxNTZDMS44ODgxIDEuNjAwMSAxLjYwMTU2IDEuODg2NjQgMS42MDE1NiAyLjI0MDFWNC45NjAxQzEuNjAxNTYgNS4zMTM1NiAxLjg4ODEgNS42MDAxIDIuMjQxNTYgNS42MDAxSDQuOTYxNTZDNS4zMTUwMiA1LjYwMDEgNS42MDE1NiA1LjMxMzU2IDUuNjAxNTYgNC45NjAxVjIuMjQwMUM1LjYwMTU2IDEuODg2NjQgNS4zMTUwMiAxLjYwMDEgNC45NjE1NiAxLjYwMDFaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00Ljk2MTU2IDEwLjM5OTlIMi4yNDE1NkMxLjg4ODEgMTAuMzk5OSAxLjYwMTU2IDEwLjY4NjQgMS42MDE1NiAxMS4wMzk5VjEzLjc1OTlDMS42MDE1NiAxNC4xMTM0IDEuODg4MSAxNC4zOTk5IDIuMjQxNTYgMTQuMzk5OUg0Ljk2MTU2QzUuMzE1MDIgMTQuMzk5OSA1LjYwMTU2IDE0LjExMzQgNS42MDE1NiAxMy43NTk5VjExLjAzOTlDNS42MDE1NiAxMC42ODY0IDUuMzE1MDIgMTAuMzk5OSA0Ljk2MTU2IDEwLjM5OTlaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik0xMy43NTg0IDEuNjAwMUgxMS4wMzg0QzEwLjY4NSAxLjYwMDEgMTAuMzk4NCAxLjg4NjY0IDEwLjM5ODQgMi4yNDAxVjQuOTYwMUMxMC4zOTg0IDUuMzEzNTYgMTAuNjg1IDUuNjAwMSAxMS4wMzg0IDUuNjAwMUgxMy43NTg0QzE0LjExMTkgNS42MDAxIDE0LjM5ODQgNS4zMTM1NiAxNC4zOTg0IDQuOTYwMVYyLjI0MDFDMTQuMzk4NCAxLjg4NjY0IDE0LjExMTkgMS42MDAxIDEzLjc1ODQgMS42MDAxWiIgZmlsbD0iI2ZmZiIvPgo8cGF0aCBkPSJNNCAxMkwxMiA0TDQgMTJaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00IDEyTDEyIDQiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIvPgo8L3N2Zz4K&logoColor=ffffff)](https://zread.ai/looplj/axonhub)

---

## 🎯 デモ

[デモインスタンス](https://axonhub.onrender.com)でAxonHubをお試しください！

**注意**：デモインスタンスでは現在、ZhipuとOpenRouterの無料モデルが設定されています。

### デモアカウント

- **メールアドレス**: demo@example.com
- **パスワード**: 12345678

---

## ⭐ 機能

### 📸 スクリーンショット

AxonHubの動作画面をご覧ください：

<table>
  <tr>
    <td align="center">
      <a href="docs/screenshots/axonhub-dashboard.png">
        <img src="docs/screenshots/axonhub-dashboard.png" alt="System Dashboard" width="250"/>
      </a>
      <br/>
      システムダッシュボード
    </td>
    <td align="center">
      <a href="docs/screenshots/axonhub-channels.png">
        <img src="docs/screenshots/axonhub-channels.png" alt="Channel Management" width="250"/>
      </a>
      <br/>
      チャネル管理
    </td>
    <td align="center">
      <a href="docs/screenshots/axonhub-model-price.png">
        <img src="docs/screenshots/axonhub-model-price.png" alt="Model Price" width="250"/>
      </a>
      <br/>
      モデル料金
    </td>
  </tr>
  <tr>
  <td align="center">
      <a href="docs/screenshots/axonhub-models.png">
        <img src="docs/screenshots/axonhub-models.png" alt="Models" width="250"/>
      </a>
      <br/>
      モデル
    </td>
    <td align="center">
      <a href="docs/screenshots/axonhub-trace.png">
        <img src="docs/screenshots/axonhub-trace.png" alt="Trace Viewer" width="250"/>
      </a>
      <br/>
      トレースビューア
    </td>
    <td align="center">
      <a href="docs/screenshots/axonhub-requests.png">
        <img src="docs/screenshots/axonhub-requests.png" alt="Request Monitoring" width="250"/>
      </a>
      <br/>
      リクエストモニタリング
    </td>
  </tr>
</table>

---

### 🚀 APIタイプ

| APIタイプ             | ステータス     | 説明                    | ドキュメント                                     |
| -------------------- | ---------- | ------------------------------ | -------------------------------------------- |
| **テキスト生成**  | ✅ 完了    | 会話インターフェース       | [OpenAI API](docs/en/api-reference/openai-api.md), [Anthropic API](docs/en/api-reference/anthropic-api.md), [Gemini API](docs/en/api-reference/gemini-api.md) |
| **画像生成** | ✅ 完了 | 画像生成               | [Image Generation](docs/en/api-reference/image-generation.md) |
| **リランク**           | ✅ 完了    | 結果のランキング                | [Rerank API](docs/en/api-reference/rerank-api.md) |
| **エンベディング**        | ✅ 完了    | ベクトルエンベディング生成    | [Embedding API](docs/en/api-reference/embedding-api.md) |
| **リアルタイム**         | 📝 予定    | リアルタイム会話機能 | -                                            |

---

### 🤖 対応プロバイダー

| プロバイダー               | ステータス     | 対応モデル             | 互換API |
| ---------------------- | ---------- | ---------------------------- | --------------- |
| **OpenAI**             | ✅ 完了    | GPT-4, GPT-4o, GPT-5など   | OpenAI, Anthropic, Gemini, Embedding, Image Generation |
| **Anthropic**          | ✅ 完了    | Claude 3.5, Claude 3.0など | OpenAI, Anthropic, Gemini |
| **Zhipu AI**           | ✅ 完了    | GLM-4.5, GLM-4.5-airなど   | OpenAI, Anthropic, Gemini |
| **Moonshot AI (Kimi)** | ✅ 完了    | kimi-k2など                | OpenAI, Anthropic, Gemini |
| **DeepSeek**           | ✅ 完了    | DeepSeek-V3.1など          | OpenAI, Anthropic, Gemini |
| **ByteDance Doubao**   | ✅ 完了    | doubao-1.6など             | OpenAI, Anthropic, Gemini, Image Generation |
| **Gemini**             | ✅ 完了    | Gemini 2.5など             | OpenAI, Anthropic, Gemini, Image Generation |
| **Fireworks**          | ✅ 完了    | MiniMax-M2.5, GLM-5, Kimi K2.5など | OpenAI |
| **Jina AI**            | ✅ 完了    | Embeddings, Rerankerなど   | Jina Embedding, Jina Rerank |
| **OpenRouter**         | ✅ 完了    | 各種モデル               | OpenAI, Anthropic, Gemini, Image Generation |
| **ZAI**                | ✅ 完了    | -                            | Image Generation |
| **AWS Bedrock**        | 🔄 テスト中 | Claude on AWS                | OpenAI, Anthropic, Gemini |
| **Google Cloud**       | 🔄 テスト中 | Claude on GCP                | OpenAI, Anthropic, Gemini |
| **NanoGPT**            | ✅ 完了    | 各種モデル、画像生成    | OpenAI, Anthropic, Gemini, Image Generation |

---

## 🚀 クイックスタート

### 30秒でローカル起動

```bash
# ダウンロードして展開（macOS ARM64の例）
curl -sSL https://github.com/looplj/axonhub/releases/latest/download/axonhub_darwin_arm64.tar.gz | tar xz
cd axonhub_*

# SQLiteで実行（デフォルト）
./axonhub

# http://localhost:8090 を開く
# 初回起動時：セットアップウィザードに従ってシステムを初期化してください（管理者アカウントの作成、パスワードは6文字以上）
```

以上です！あとはAIチャネルを設定し、AxonHub経由でモデルの呼び出しを開始できます。

### コード変更ゼロの移行例

**既存のコードはそのまま動作します。** SDKの接続先をAxonHubに向けるだけです：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8090/v1",  # AxonHubに接続
    api_key="your-axonhub-api-key"        # AxonHubのAPIキーを使用
)

# OpenAI SDKでClaudeを呼び出し！
response = client.chat.completions.create(
    model="claude-3-5-sonnet",  # またはgpt-4, gemini-pro, deepseek-chat...
    messages=[{"role": "user", "content": "Hello!"}]
)
```

モデルの切り替えは1行変更するだけ：`model="gpt-4"` → `model="claude-3-5-sonnet"`。SDKの変更は不要です。

### Renderへのワンクリックデプロイ

[Render](https://render.com)でAxonHubをワンクリックで無料デプロイ。

<div>

<a href="https://render.com/deploy?repo=https://github.com/looplj/axonhub">
  <img src="https://render.com/images/deploy-to-render-button.svg" alt="Deploy to Render">
</a>

</div>

---

## 🚀 デプロイガイド

### 💻 パーソナルコンピューターへのデプロイ

個人開発者や小規模チームに最適。複雑な設定は不要です。

#### ダウンロードと実行

1. **最新リリースをダウンロード** - [GitHub Releases](https://github.com/looplj/axonhub/releases)から

   - お使いのオペレーティングシステムに合ったバージョンを選択してください：

2. **展開して実行**

   ```bash
   # ダウンロードしたファイルを展開
   unzip axonhub_*.zip
   cd axonhub_*

   # 実行権限を追加（Linux/macOSのみ）
   chmod +x axonhub

   # 直接実行 - デフォルトのSQLiteデータベース

   # AxonHubをシステムにインストール
   sudo ./install.sh

   # AxonHubサービスを開始
   ./start.sh

   # AxonHubサービスを停止
   ./stop.sh
   ```

3. **アプリケーションにアクセス**
   ```
   http://localhost:8090
   ```

---

### 🖥️ サーバーデプロイ

本番環境、高可用性、エンタープライズデプロイ向け。

#### データベースサポート

AxonHubは、異なる規模のデプロイニーズに対応するために複数のデータベースをサポートしています：

| データベース       | サポートバージョン | 推奨シナリオ                             | 自動マイグレーション | リンク                                                       |
| -------------- | ------------------ | ------------------------------------------------ | -------------- | ----------------------------------------------------------- |
| **TiDB Cloud** | Starter            | サーバーレス、無料プラン、オートスケール                | ✅ サポート   | [TiDB Cloud](https://www.pingcap.com/tidb-cloud-starter/)   |
| **TiDB Cloud** | Dedicated          | 分散デプロイ、大規模運用              | ✅ サポート   | [TiDB Cloud](https://www.pingcap.com/tidb-cloud-dedicated/) |
| **TiDB**       | V8.0+              | 分散デプロイ、大規模運用              | ✅ サポート   | [TiDB](https://tidb.io/)                                    |
| **Neon DB**    | -                  | サーバーレス、無料プラン、オートスケール                | ✅ サポート   | [Neon DB](https://neon.com/)                                |
| **PostgreSQL** | 15+                | 本番環境、中〜大規模デプロイ | ✅ サポート   | [PostgreSQL](https://www.postgresql.org/)                   |
| **MySQL**      | 8.0+               | 本番環境、中〜大規模デプロイ | ✅ サポート   | [MySQL](https://www.mysql.com/)                             |
| **SQLite**     | 3.0+               | 開発環境、小規模デプロイ       | ✅ サポート   | [SQLite](https://www.sqlite.org/index.html)                 |

#### 設定

AxonHubは、環境変数によるオーバーライドをサポートするYAML設定ファイルを使用します：

```yaml
# config.yml
server:
  port: 8090
  name: "AxonHub"
  debug: false

db:
  dialect: "tidb"
  dsn: "<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true&parseTime=true&multiStatements=true&charset=utf8mb4"

log:
  level: "info"
  encoding: "json"
```

環境変数：

```bash
AXONHUB_SERVER_PORT=8090
AXONHUB_DB_DIALECT="tidb"
AXONHUB_DB_DSN="<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true&parseTime=true&multiStatements=true&charset=utf8mb4"
AXONHUB_LOG_LEVEL=info
```

詳細な設定手順については、[設定ドキュメント](docs/en/deployment/configuration.md)を参照してください。

#### Docker Composeデプロイ

```bash
# プロジェクトをクローン
git clone https://github.com/looplj/axonhub.git
cd axonhub

# 環境変数を設定
export AXONHUB_DB_DIALECT="tidb"
export AXONHUB_DB_DSN="<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true&parseTime=true&multiStatements=true&charset=utf8mb4"

# サービスを開始
docker-compose up -d

# ステータスを確認
docker-compose ps
```

#### Helm Kubernetesデプロイ

公式Helm chartを使用して、Kubernetes上にAxonHubをデプロイします：

```bash
# クイックインストール
git clone https://github.com/looplj/axonhub.git
cd axonhub
helm install axonhub ./deploy/helm

# 本番デプロイ
helm install axonhub ./deploy/helm -f ./deploy/helm/values-production.yaml

# AxonHubにアクセス
kubectl port-forward svc/axonhub 8090:8090
# http://localhost:8090 にアクセス
```

**主要な設定オプション：**

| パラメータ | 説明 | デフォルト |
|-----------|-------------|---------|
| `axonhub.replicaCount` | レプリカ数 | `1` |
| `axonhub.dbPassword` | DBパスワード | `axonhub_password` |
| `postgresql.enabled` | 組み込みPostgreSQL | `true` |
| `ingress.enabled` | Ingressの有効化 | `false` |
| `persistence.enabled` | データ永続化 | `false` |

詳細な設定とトラブルシューティングについては、[Helm Chartドキュメント](deploy/helm/README.md)を参照してください。

#### 仮想マシンデプロイ

[GitHub Releases](https://github.com/looplj/axonhub/releases)から最新リリースをダウンロードしてください

```bash
# 展開して実行
unzip axonhub_*.zip
cd axonhub_*

# 環境変数を設定
export AXONHUB_DB_DIALECT="tidb"
export AXONHUB_DB_DSN="<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true&parseTime=true&multiStatements=true&charset=utf8mb4"

sudo ./install.sh

# 設定ファイルを確認
axonhub config check

# サービスを開始
#  簡便のため、ヘルパースクリプトでAxonHubを管理することを推奨します：

# 開始
./start.sh

# 停止
./stop.sh
```

---

## 📖 使用ガイド

### 統合APIの概要

AxonHubは、OpenAI Chat CompletionsとAnthropic Messages APIの両方をサポートする統合APIゲートウェイを提供します。これにより以下が可能になります：

- **OpenAI APIでAnthropicモデルを呼び出し** - OpenAI SDKを使いながらClaudeモデルにアクセス
- **Anthropic APIでOpenAIモデルを呼び出し** - Anthropicのネイティブフォーマットを使いながらGPTモデルにアクセス
- **Gemini APIでOpenAIモデルを呼び出し** - Geminiのネイティブフォーマットを使いながらGPTモデルにアクセス
- **自動API変換** - AxonHubがフォーマット変換を自動的に処理
- **コード変更ゼロ** - 既存のOpenAIまたはAnthropicクライアントコードがそのまま動作

### 1. 初期セットアップ

1. **管理画面にアクセス**

   ```
   http://localhost:8090
   ```

2. **AIプロバイダーの設定**

   - 管理画面でAPIキーを追加
   - 接続テストで正しい設定を確認

3. **ユーザーとロールの作成**
   - 権限管理のセットアップ
   - 適切なアクセス権限を割り当て

### 2. チャネル設定

管理画面でAIプロバイダーチャネルを設定します。モデルマッピング、パラメータオーバーライド、トラブルシューティングを含むチャネル設定の詳細については、[チャネル設定ガイド](docs/en/guides/channel-management.md)を参照してください。

### 3. モデル管理

AxonHubは、モデルアソシエーションを通じて抽象モデルを特定のチャネルおよびモデル実装にマッピングする柔軟なモデル管理システムを提供します。これにより以下が可能になります：

- **統一モデルインターフェース** - チャネル固有の名前ではなく、抽象モデルID（例：`gpt-4`、`claude-3-opus`）を使用
- **インテリジェントなチャネル選択** - アソシエーションルールとロードバランシングに基づいて、最適なチャネルに自動ルーティング
- **柔軟なマッピング戦略** - 正確なチャネル-モデルマッチング、正規表現パターン、タグベースの選択をサポート
- **優先度ベースのフォールバック** - 自動フェイルオーバーのために優先度付きの複数アソシエーションを設定

モデル管理の包括的な情報（アソシエーションタイプ、設定例、ベストプラクティスを含む）については、[モデル管理ガイド](docs/en/guides/model-management.md)を参照してください。

### 4. APIキーの作成

AxonHubでアプリケーションを認証するためのAPIキーを作成します。各APIキーには、以下を定義する複数のプロファイルを設定できます：

- **モデルマッピング** - 完全一致または正規表現パターンを使用して、ユーザーがリクエストしたモデルを実際に利用可能なモデルに変換
- **チャネル制限** - チャネルIDまたはタグによって、APIキーが使用できるチャネルを制限
- **モデルアクセス制御** - 特定のプロファイルを通じてアクセス可能なモデルを制御
- **プロファイル切り替え** - 異なるプロファイルをアクティブにすることで、動作をオンザフライで変更

APIキープロファイルの詳細（設定例、バリデーションルール、ベストプラクティスを含む）については、[APIキープロファイルガイド](docs/en/guides/api-key-profiles.md)を参照してください。

### 5. AIコーディングツール連携

詳細なセットアップ手順、トラブルシューティング、およびAxonHubモデルプロファイルとの組み合わせに関するヒントについては、以下の専用ガイドを参照してください：
- [OpenCode連携ガイド](docs/en/guides/opencode-integration.md)
- [Claude Code連携ガイド](docs/en/guides/claude-code-integration.md)
- [Codex連携ガイド](docs/en/guides/codex-integration.md)

---

### 6. SDKの使用方法

SDKの詳細な使用例とコードサンプルについては、APIドキュメントを参照してください：
- [OpenAI API](docs/en/api-reference/openai-api.md)
- [Anthropic API](docs/en/api-reference/anthropic-api.md)
- [Gemini API](docs/en/api-reference/gemini-api.md)

## 🛠️ 開発ガイド

詳細な開発手順、アーキテクチャ設計、コントリビューションガイドラインについては、[docs/en/development/development.md](docs/en/development/development.md)を参照してください。

---

## 🤝 謝辞

- 🙏 [musistudio/llms](https://github.com/musistudio/llms) - LLM変換フレームワーク、インスピレーションの源
- 🎨 [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin) - 管理画面テンプレート
- 🔧 [99designs/gqlgen](https://github.com/99designs/gqlgen) - GraphQLコード生成
- 🌐 [gin-gonic/gin](https://github.com/gin-gonic/gin) - HTTPフレームワーク
- 🗄️ [ent/ent](https://github.com/ent/ent) - ORMフレームワーク
- 🔧 [air-verse/air](https://github.com/air-verse/air) - Goサービスの自動リロード
- ☁️ [Render](https://render.com) - デモをホスティングする無料クラウドデプロイプラットフォーム
- 🗃️ [TiDB Cloud](https://www.pingcap.com/tidb-cloud/) - デモデプロイ用のサーバーレスデータベースプラットフォーム

---

## 📄 ライセンス

このプロジェクトは複数のライセンス（Apache-2.0およびLGPL-3.0）の下でライセンスされています。詳細なライセンスの概要と条項については、[LICENSE](LICENSE)ファイルを参照してください。

---

<div align="center">

**AxonHub** - オールインワンAI開発プラットフォーム、AI開発をよりシンプルに

[🏠 ホームページ](https://github.com/looplj/axonhub) • [📚 ドキュメント](https://deepwiki.com/looplj/axonhub) • [🐛 問題報告](https://github.com/looplj/axonhub/issues)

AxonHubチームが ❤️ を込めて開発

</div>
