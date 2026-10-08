# cpa-quota-primer

CLIProxyAPI（CPA）原生插件：按 cron 定时检查 Codex、Claude OAuth 订阅账户的**用量窗口**，只对存在**未启动窗口**且没有**耗尽窗口**的账户发送一次最小**预热请求**，让窗口在你开始工作前开始计时。

插件不保存任何预热状态。每轮都重新查询上游额度，再按下表判定。

| 提供方 | 目标窗口 | 判定为未启动 | 跳过 |
| --- | --- | --- | --- |
| Codex | 五小时窗口（18000s）、周窗口（604800s），按 `limit_window_seconds` 识别 | `used_percent == 0` 且 `\|reset_after_seconds - limit_window_seconds\| <= 10` | 任一窗口 `used_percent >= 100`；字段缺失或无窗口时判为未知 |
| Claude | 仅 `five_hour` | `utilization == 0` 且 `resets_at == null` | `five_hour` 或 `seven_day` 的 `utilization >= 100`；`five_hour` 缺失、`resets_at` 字段缺失或无法解析时判为未知 |

只处理 `account_type` 为 `oauth`、未禁用、所属提供方已启用的账户。API key 账户永远不参与。

## 安装

1. 构建动态库（需要 CGO 和 C 编译器；Windows 用 MinGW gcc），文件名必须是插件 ID：

   ```sh
   go build -buildmode=c-shared -o /path/to/cpa/plugins/cpa-quota-primer.so ./cmd/plugin   # Linux
   go build -buildmode=c-shared -o C:/path/to/cpa/plugins/cpa-quota-primer.dll ./cmd/plugin   # Windows
   ```

   macOS 用 `.dylib`。构建同时生成的 `.h` 文件可以删除。

2. 在 CPA 配置中启用插件：

   ```yaml
   plugins:
     enabled: true
     dir: "plugins"
     configs:
       cpa-quota-primer:
         enabled: true
         cron: "30 8 * * *"          # 必填，标准 5 字段
         timezone: "Asia/Tokyo"      # 可选，IANA 名称
         management:
           base_url: "http://127.0.0.1:8317"  # 可选，默认如左
           key: "<管理密钥明文>"               # 必填
         providers:
           codex:  { enabled: true,  model: "" }
           claude: { enabled: false, model: "" }
   ```

3. 重启 CPA。宿主日志出现 `pluginhost: plugin registered plugin_id=cpa-quota-primer` 后，插件立即执行一轮预热，之后按 cron 执行。

## 配置

| 配置项 | 说明 |
| --- | --- |
| `cron` | 标准 5 字段 cron（分 时 日 月 周）。拒绝 6 字段（秒级）、`@daily` / `@every` 等描述符和 `CRON_TZ=` / `TZ=` 前缀，避免在 10 秒容错期内重复预热。 |
| `timezone` | cron 使用的 IANA 时区。未配置时用宿主本地时区；宿主时区无法确定时，Go 运行时回退为 UTC。 |
| `management.base_url` | CPA 管理 API 地址，默认 `http://127.0.0.1:8317`。走本机回环，不经过公网代理，也不受"禁止远程管理"影响。CPA 监听其他端口时改这里；Docker 部署填容器内端口。 |
| `management.key` | 管理密钥明文。CPA 配置里只保存哈希，插件读不到，所以必须在这里填写。 |
| `providers.<codex\|claude>.enabled` | 提供方开关。启用后，该提供方下全部 OAuth 账户自动参与。 |
| `providers.<codex\|claude>.model` | 预热使用的模型。留空时使用该账户模型列表中第一个名称不含 `image` 的模型。 |

配置非法时插件拒绝加载，宿主日志出现 `plugin.register failed: <原因>`。重配置非法时，正在运行的调度保持不变。

## 行为

- 每个账户每轮写一行日志 `quota primer account result`，字段包括 `provider`、`account`（email 或文件名）、`decision`、`reason`；发送预热请求时还有 `model`、`warmup_status`、`warmup_error`。`decision` 取值：`not_started`（已发送预热）、`started`、`exhausted`、`unknown`（附带原始窗口字段 `windows`）、`query_failed`、`skipped`。
- 预热请求经宿主 `host.model.execute` 固定发往该账户（Codex 走 Responses、Claude 走 Messages，输入 `hi`，非流式），与正常流量共用 CPA 的执行器、token 刷新、代理和请求日志，失败时不切换到其他账户。
- Claude 预热后立即复查一次额度。若 `five_hour.resets_at` 仍为 null，记录警告（`recheck` 字段），下一轮照常重试。Codex 不复查：请求后 1–3 秒内 `reset_after_seconds` 仍等于完整时长，复查会误报。
- 同一时刻最多一轮。上一轮未结束时到来的 cron 触发直接跳过，并记录 `trigger skipped`。CPA 停机期间错过的触发不补发。
- 单次预热请求最长等待 2 分钟。CPA 停止或重配置插件时，正在执行的轮次立即取消；宿主无法取消已发出的 `host.model.execute`，该请求在宿主内跑完，结果丢弃。
- 宿主先注册插件、后启动 HTTP 服务，所以启动轮在管理 API 无法连接时每秒重试，最多 2 分钟。管理 API 返回 HTTP 错误（如密钥错误的 401）时不重试，以免触发 CPA 的 IP 封禁。
- 日志不包含管理密钥、账户 token 或上游响应正文。

## 已知限制

- 额度接口（`wham/usage`、`api/oauth/usage`）及其请求头不是官方公开、承诺稳定的 API。字段变化时账户会判为 `unknown`，日志附带原始字段。
- 一次请求能否启动 Claude 五小时窗口尚未实测；若不能，该账户每轮都会再次预热并记录警告。
- Codex 周窗口未启动时的字段形态尚未实测，目前假设与五小时窗口一致。
- CPA 的 api-call 查询 Claude 额度时不刷新 token。token 过期时额度查询失败，账户被跳过，直到 CPA 通过其他流量刷新 token。
- 插件不检查账户侧额外付费设置，不提供请求级的"禁止额外计费"保证。

## 开发

```sh
go test ./...       # 无网络、无真实账户
go test -race ./...
go vet ./...
```

`cmd/plugin` 依赖 cgo，对它执行 vet / build 需要 C 编译器。开发目标为 CPA `main` 的插件 SDK（commit `54946fa3dfa29c6ca7312ac141a92cdd5e413771`，ABI 1，schema 6）。
