# CLIProxyAPI 插件能力核验

核验日期：2026-10-08。设计质询资料，不是已确认规格。只读上游文档与源码，未执行宿主、未发送模型请求。

## 原生插件与 JS handler

CLIProxyAPI 原生插件在宿主进程内运行，通过 C ABI 初始化、调用及停机。生命周期包含注册、重配置及停止；源码另有 quiesce。[官方开发文档](https://help.router-for.me/plugin/development)；[宿主源码](https://github.com/router-for-me/CLIProxyAPI/blob/main/internal/pluginhost/host.go)。

[INFERENCE] 原生插件可管理自己的后台线程或 goroutine 与定时器，并在 quiesce/shutdown 停止。宿主没有已核验的 cron/tick 服务；后台调度尚未实跑。Windows 正常停止不卸载 Go DLL 映射，不能依赖模块卸载终止后台工作。[Windows loader](https://github.com/router-for-me/CLIProxyAPI/blob/main/internal/pluginhost/loader_windows.go)。

JS handler 每次请求/响应 hook 创建 Goja VM，仅注入 console；没有常驻事件循环、定时调度或宿主模型调用桥接。其 time.AfterFunc 用于脚本执行超时，不是定时任务。因此现有 JS hook 不能在没有入站请求时自行触发预热。[注册](https://github.com/router-for-me/cpa-plugin-jshandler/blob/main/main.go)；[引擎](https://github.com/router-for-me/cpa-plugin-jshandler/blob/main/engine.go)；[拦截器](https://github.com/router-for-me/cpa-plugin-jshandler/blob/main/interceptor.go)。

## 账户选择与模型请求

- 宿主提供 host.auth.list/get/get_runtime。runtime 返回账户条目，不是完整内部 Auth 对象。[凭据回调文档](https://help.router-for.me/plugin/host-callbacks#credential-file-callbacks)；[实现](https://github.com/router-for-me/CLIProxyAPI/blob/main/internal/pluginhost/auth_callbacks.go)。
- 当前 main 的 host.model.execute 支持 auth_id，HostModelExecutionRequest.AuthID 可锁定精确凭据。ExecuteModel 转为 WithPinnedAuthID，调度器排除其他 ID；目标失败不会换另一账户。auth_id 不等同于 auth_index。[类型](https://github.com/router-for-me/CLIProxyAPI/blob/main/sdk/pluginapi/types.go)；[执行](https://github.com/router-for-me/CLIProxyAPI/blob/main/sdk/api/handlers/model_execution.go)；[调度](https://github.com/router-for-me/CLIProxyAPI/blob/main/sdk/cliproxy/auth/scheduler.go)。
- 普通模型请求经过账户池路由及受限失败切换，不能保证某个 provider 下每个账户都收到请求。[选择](https://github.com/router-for-me/CLIProxyAPI/blob/main/sdk/cliproxy/auth/conductor_selection.go)；[执行循环](https://github.com/router-for-me/CLIProxyAPI/blob/main/sdk/cliproxy/auth/conductor_execution.go)。
- 宿主回调持有插件身份，不要求存在入站 HTTP hook；模型执行器仍须已初始化。[回调](https://github.com/router-for-me/CLIProxyAPI/blob/main/internal/pluginhost/host_callbacks.go)；[Unix 桥接](https://github.com/router-for-me/CLIProxyAPI/blob/main/internal/pluginhost/host_callbacks_unix.go)。

## 窗口状态边界

未找到统一 host.quota.fetch 回调。host.auth.get_runtime 的 retry/cooldown 信息不能当订阅窗口 reset 时间。[回调类型](https://github.com/router-for-me/CLIProxyAPI/blob/main/sdk/pluginabi/types.go)；[账户类型](https://github.com/router-for-me/CLIProxyAPI/blob/main/sdk/cliproxy/auth/types.go)。

Codex/Claude 有响应头的被动 quota 信号采集；管理账户列表提供 observed_at/signals 快照，并非实时查询。模型回调可返回响应头。主动 FetchCredentialQuota 依赖 quota provider 或 quota_probe；缺少支持时返回 501。QuotaBucket.ResetTime 类型存在不代表具体提供方可用。[信号](https://github.com/router-for-me/CLIProxyAPI/blob/main/sdk/cliproxy/auth/quota_signals.go)；[管理快照](https://github.com/router-for-me/CLIProxyAPI/blob/main/internal/api/handlers/management/auth_files.go)；[主动查询](https://github.com/router-for-me/CLIProxyAPI/blob/main/internal/api/handlers/management/plugin_quota.go)。

## 版本及待核验事项

能力按 CLIProxyAPI main 核验，树查询 SHA 为 0f96f568e4dbf6f84ad7399a74b78344c5eac7e6，不代表用户安装版本。JS handler go.mod 依赖 CLIProxyAPI/v7 v7.1.70，而当前宿主源码使用 v8。[JS handler go.mod](https://github.com/router-for-me/cpa-plugin-jshandler/blob/main/go.mod)。

尚未验证实际宿主运行、每账户实时窗口信息、最小请求的窗口启动效果，以及请求级禁止额外付费的可靠机制。插件宿主调用能力不能单独证明这些要求。

## 仅订阅额度与额外计费

当前 CPA HostModelExecutionRequest 及执行路径未找到已文档化的请求级禁用 credits / extra usage 字段。AuthID 只锁定账户，不限制上游计费方式。[请求类型](https://github.com/router-for-me/CLIProxyAPI/blob/main/sdk/pluginapi/types.go)。

Claude 官方支持在 Settings > Usage 关闭 Usage credits，并明确关闭后仅可使用订阅包含用量；该机制适用于 Claude Code。插件能否读取开关状态尚未证实。[官方说明](https://support.claude.com/en/articles/12429409-manage-usage-credits-for-paid-claude-plans)。Claude fast mode 直接消耗 credits，即使订阅仍有剩余额度，因此不能仅检查剩余额度来保证仅订阅使用。[fast mode](https://code.claude.com/docs/en/fast-mode)。

Codex 官方说明先使用订阅额度，达到计划限制后消耗已有 credit balance。关闭 automatic reload 只禁止自动购买，不等于禁止消费已有 credits；并发用量还可能使正余额任务产生负余额。[credits 指南](https://help.openai.com/en/articles/12642688-using-credits-for-flexible-usage-in-chatgpt-personal-plans)。

未找到 CPA 所用原生 Codex OAuth 通道的服务端强制仅订阅参数或普遍账户开关。OpenAI 参与第三方应用的订阅共享机制有单独 credits opt-in，但不能外推到 CPA 通道。[应用共享指南](https://help.openai.com/en/articles/20001542-using-your-chatgpt-plan-in-other-apps-and-sites)。

[INFERENCE] 额度预查与执行不是原子授权，不能作为零额外计费保证。严格禁止额外费用的要求需要可验证的上游计费保护；缺少保护时的处理策略仍待用户确认。未登录账户，未改变计费设置，未发送请求。

## 即时触发能力（调查）

本节核验上游 `main`，固定 commit **`54946fa3dfa29c6ca7312ac141a92cdd5e413771`**，比前文使用的 `0f96f568e4dbf6f84ad7399a74b78344c5eac7e6` 更新。下列“不存在”仅指此版本公开插件能力／ABI，不指宿主内部没有相应状态。未执行宿主、未使用凭据、未查询真实额度、未发送模型请求。[P1][P2]

### 机制清单

| 机制 | 存在／不存在／未知 | 签名、方法或路径 | 已核验边界 |
| --- | --- | --- | --- |
| 插件自定义 HTTP 管理动作 | **存在** | `ManagementAPI.RegisterManagement(context.Context, ManagementRegistrationRequest) (ManagementRegistrationResponse, error)`；返回 `Routes []ManagementRoute`；处理器 `ManagementHandler.HandleManagement(context.Context, ManagementRequest) (ManagementResponse, error)`；ABI 为 `management.register` / `management.handle` | 可注册例如 `POST /v0/management/plugins/<pluginID>/run`，这是文档支持的动作形态，不是宿主预置的预热 endpoint。需要管理 key，并受管理接口可达性／远程访问设置约束；精确 method + path 匹配，不支持 `:`、`*` 动态路由，不能覆盖宿主保留路由。[P2][P3] |
| 任意业务 HTTP 路由 | **不存在（注册接口）** | `ManagementRoute` 被归一至 `/v0/management/...`；`ResourceRoute` 被归一至 `/v0/resource/plugins/<pluginID>/...` | 已公开注册能力仅涵盖这两类路径；不能由该接口注册任意 `/v1/...` 路由。进程内自己监听端口不是 CPA 提供的路由能力，本节未调查或验证。[P2][P3] |
| 管理 UI 的插件入口／动作 | **存在（菜单入口）；不存在（通用 run-now 按钮契约）** | `Resources []ResourceRoute`，包含 `Path`、`Menu`、`Description`；`ListPlugins` 返回插件菜单 | 宿主暴露插件资源页菜单；插件可在自己的页面调用已认证管理动作。资源页 GET 本身不做管理鉴权，不应直接执行敏感动作。同源页面可能复用管理中心 storage 中的 key，跨源不能假定可取。未验证用户安装版本的前端渲染；没有找到宿主为所有插件自动生成“立即运行”动作的契约。[P3][P4] |
| 插件 CLI 命令／flag | **存在** | `CommandLinePlugin.RegisterCommandLine(...)` / `ExecuteCommandLine(...)`；ABI `command_line.register` / `command_line.execute` | 这是 CPA 进程启动时解析的 flag，不是向已运行宿主发送命令的 IPC。`cmd/server/main.go` 在启动服务之前执行命中的插件命令，处理完成后 return；不能据此保证命令里已有可用的宿主模型执行器。[P2][P5] |
| 账号新增／更新事件订阅 | **不存在（原生插件 ABI）；存在（宿主 Go SDK 内部 Hook）** | `sdk/cliproxy/auth/conductor.go` 的 `Hook`：`OnAuthRegistered(ctx, *Auth)`、`OnAuthUpdated(ctx, *Auth)`、`OnResult(ctx, Result)` | `pluginapi.Capabilities` 没有该 Hook，`pluginabi` 没有 auth-change subscribe／通知方法；不能将给自建宿主使用的 Go Hook 当作 DLL 插件可注册的事件。`AuthProvider` 的 parse／login／refresh 处理插件自己拥有的 provider，不是监听所有 Codex／Claude 账号变化。[P2][P6] |
| 额度／cooldown 变化、429 清除事件 | **不存在（专用插件通知／订阅）** | 插件方法表没有 quota-change、cooldown-change、429-cleared 或 usage-reset 事件；存在 `host.routing.reset_cooldown` | 后者是插件主动要求清除 CPA 路由状态，不是上游订阅额度 reset，也不是通知。宿主 `POST /v0/management/reset-quota` 同样仅清除本地 quota／cooldown；不能把调用成功当成用户已使用重置卡。[P2][P7] |
| 被动 quota 观察事件／快照 | **不存在（订阅）；存在（管理读取）** | `QuotaState.ObserveResponseHeadersForProvider(...)`；`GET /v0/management/auth-files` 的每账户 `quota.observed_at` / `quota.signals`，可另有 `model_quotas` | 快照来自最近一次有 quota 信号的 CPA 上游响应，不是主动 usage 查询，也没有推送给插件。新快照替换旧快照；没有信号的响应保留旧值，因此可陈旧。采集最多 64 个头，单值最多 512 字节，只保留有效最后一个值；观察字段不改变 scheduler cooldown。[P8][P9] |
| `host.auth.*` 读取 quota 快照 | **不存在（当前返回类型）；存在（有限运行时状态）** | `host.auth.list` / `host.auth.get_runtime` 返回 `HostAuthFileEntry`；`host.auth.get` 返回物理凭据 JSON | `HostAuthFileEntry` 有 `Status`、`Unavailable`、`NextRetryAfter`、`UpdatedAt` 等，但没有 `Quota`、`Signals`、`ObservedAt` 或完整 `ModelStates`。`get` 也不是完整内存 Auth；要读上述管理快照，需要另走已认证管理 HTTP 接口，不能假定插件自动拥有管理 key。[P2][P9] |
| `QuotaProvider`／`QuotaBucket` | **存在（插件提供能力）；不存在（通用订阅／host quota 回调）** | `DescribeQuota(ctx, QuotaDescribeRequest)`、`FetchQuota(ctx, QuotaFetchRequest)`、`ResetQuota(ctx, QuotaResetRequest)`；`QuotaBucket{Window, RemainingFraction, ResetTime, Description}` | 是宿主调用额度提供插件的接口，不是插件订阅宿主被动观察。管理查询可调用已注册 provider 或 declarative probe；无支持则 501。`POST /quota/reset` 与 `/plugins/:id/quota/reset` 可调用支持 reset 的插件；这不能证明内置 Codex／Claude 支持购买／使用重置卡，也不会自动把官网发生的 reset 广播给其他插件。[P2][P10] |
| 凭据文件变化 | **不存在（通用插件通知）；存在（宿主 watcher）** | `internal/watcher/dispatcher.go` 的 `AuthUpdate` 队列与 add／modify／delete 分发 | watcher 服务宿主账号同步，不是原生插件订阅接口。外部官网额度 reset 是否修改 CPA 本地凭据文件：**未知**，不能把文件变化当 reset 的可靠代理信号。[P2][P11] |
| 普通用户流量的 HTTP 响应 hook | **存在** | `ResponseInterceptor.InterceptResponse(ctx, ResponseInterceptRequest)`，ABI `response.intercept_after`；流式 `StreamChunkInterceptor.InterceptStreamChunk(...)`，ABI `response.intercept_stream_chunk` | 成功响应结构包含 `ResponseHeaders http.Header`，非流式另含 `StatusCode`；流式在 `ChunkIndex=-1` 的 header-init 及后续 chunk 提供 headers。handler 传入 raw 上游头的克隆，不依赖最终下游头透传开关。只在该请求已经得到响应后观察，不覆盖无流量空闲期间，也不是通用失败响应／429 的头 hook。[P2][P12] |
| request-before／after、完成／usage 回调 | **存在（流量生命周期）；不存在（reset 专用信号）** | `request.intercept_before` / `request.intercept_after`；`RequestLifecyclePlugin.HandleRequestComplete(ctx, RequestCompletion)`；`usage.handle` | request-after 指鉴权选择之后、执行之前，没有上游 quota 响应头。`RequestCompletion` 异步给终态、状态码、错误与 metadata，没有响应头；usage 是 token 用量记录，不是订阅额度恢复事件。不能把任意 `on_after_*` 名称都解释为可收 quota headers。[P2][P12] |
| 上游 WebSocket 响应事件 | **存在** | `WebSocketResponseObserver.ObserveWebSocketResponseEvent(ctx, WebSocketResponseEvent)`，含 `AuthID`、`Provider`、`EventType`、`Payload`；ABI `response.observe_websocket_event` | 可看正在执行的 WebSocket 模型请求事件；Codex `codex.rate_limits` 被宿主另行归一为 quota headers。该观察器不是后台订阅官网 reset 的连接；无进行中的请求不保证收到任何额度事件。[P2][P12][P13] |
| 配置热更新／reconfigure | **存在** | ABI `plugin.reconfigure` 请求含 `config_yaml`；`PUT` / `PATCH /v0/management/plugins/:id/config` | 管理 handler 保存插件配置；配置 watcher／Service 更新流程最终调用 `Host.ApplyConfig`，已注册插件改走 reconfigure。插件可在处理回调时执行自有逻辑，但“配置字段触发运行”不是宿主内建语义；重配可来自其他配置变更，不能把回调本身当唯一一次用户指令或额度 reset 事件。watcher 对完全相同文件内容按 hash 跳过，没有同步完成／低延迟触发保证。[P4][P11][P14] |

### 响应头具体包含什么

- **Codex**：CPA 接受 `X-Codex-Primary-*`／`Secondary-*` 等窗口头，以及其他 namespaced limits。窗口信号包含 `used-percent`、`window-minutes`、`reset-after-seconds`、`reset-at`；另有 `allowed`、`limit-reached`、`plan-type`、`active-limit`、`credits-*`。WebSocket `codex.rate_limits` 的窗口、credits 和 additional limits 可转换成相同用途的 `X-Codex-*` 头表示；HTTP 附加桶与 WebSocket 附加桶命名不完全相同。CPA 的采集器只保存字符串，不保证每个响应都有所有字段，也不把字段解释为“刚使用重置卡”。`X-Ratelimit-*` 对 Codex 的规则是未来兼容保留，源码注释说当前观察到的 Codex 响应不携带这组头。[P8][P13]
- **Claude**：官方确认 `anthropic-ratelimit-unified-*` 在成功响应显示 claude.ai 计划额度，在 429 区分计划限制／spend cap 与临时 throttle；`retry-after` 是重试等待秒数，不是五小时窗口起点。CPA 测试样例含 `...-5h-status/utilization/reset`、`...-7d-status/utilization/reset`，以及总体 `status/reset`、`representative-claim`、`overage-status`、`overage-disabled-reason` 等。**这些具体子头与样值是 CPA 源码测试证据，不是 Anthropic 稳定后端 schema**：例如样例 utilization 为 `0.53`、reset 为 Unix 秒字符串，不能直接套用 HTTP usage 查询中 0–100 的 `utilization` 与 RFC3339 `resets_at`。该采集器没有定义 null／未启动或显式 reset 的语义。[P8][P15]

**本节事实结论**：已核验的即时人工入口是插件自定义管理 HTTP 动作；菜单／资源页可以承载该动作。未找到 CPA 原生插件自动接收“用户刚在官网使用重置卡”的专用事件。被动 headers 只在已有模型流量时更新；`QuotaProvider.ResetQuota` 只处理经该插件发起的 reset 调用，不等于识别官网操作。[P2][P3][P8][P10] [INFERENCE] 对不经 CPA 的官网 reset，没有新流量、主动查询或外部人工信号时，上述公开接口不足以保证立即知晓。这是能力边界，不是触发方式的设计决定。

### 来源（本节新增）

本文件此前没有编号来源；本节从 [P1] 编号。以下 CPA 源码均固定于 `54946fa3dfa29c6ca7312ac141a92cdd5e413771`；在线文档没有对应 commit 保证。

- [P1] CPA `main` commit 查询：<https://api.github.com/repos/router-for-me/CLIProxyAPI/commits/54946fa3dfa29c6ca7312ac141a92cdd5e413771>。
- [P2] CPA 插件能力、字段与 ABI 方法全集：[`sdk/pluginapi/types.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/sdk/pluginapi/types.go)；[`sdk/pluginabi/types.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/sdk/pluginabi/types.go)。
- [P3] 官方插件 Management API 文档：<https://help.router-for.me/plugin/management-api>；[`internal/pluginhost/management.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/pluginhost/management.go)；[`internal/api/server_management.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/api/server_management.go)。
- [P4] 菜单返回、配置 PUT／PATCH：[`internal/api/handlers/management/plugins.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/api/handlers/management/plugins.go)。
- [P5] 官方 CLI 插件文档：<https://help.router-for.me/plugin/command-line-plugin>；启动执行顺序：[`cmd/server/main.go#L703-L709`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/cmd/server/main.go#L703-L709)。
- [P6] 宿主内部 auth Hook：[`sdk/cliproxy/auth/conductor.go#L109-L129`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/sdk/cliproxy/auth/conductor.go#L109-L129)。
- [P7] 本地路由状态 reset：[`internal/api/handlers/management/quota.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/api/handlers/management/quota.go)。
- [P8] 被动 headers 采集与测试样例：[`sdk/cliproxy/auth/quota_signals.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/sdk/cliproxy/auth/quota_signals.go)；[`quota_signals_test.go#L69-L100`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/sdk/cliproxy/auth/quota_signals_test.go#L69-L100)。
- [P9] 管理快照与 host.auth 返回组装：[`internal/api/handlers/management/auth_files.go#L670-L676`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/api/handlers/management/auth_files.go#L670-L676)；[`internal/pluginhost/auth_callbacks.go#L422-L503`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/pluginhost/auth_callbacks.go#L422-L503)。
- [P10] quota fetch／reset 调度与无 provider 边界：[`internal/api/handlers/management/plugin_quota.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/api/handlers/management/plugin_quota.go)。
- [P11] watcher auth 队列／配置 hash 判定：[`internal/watcher/dispatcher.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/watcher/dispatcher.go)；[`internal/watcher/config_reload.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/watcher/config_reload.go)。
- [P12] 请求、响应、WebSocket 与流式 hook 组装：[`sdk/api/handlers/handlers_interceptors.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/sdk/api/handlers/handlers_interceptors.go)；[`sdk/api/handlers/handlers_stream.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/sdk/api/handlers/handlers_stream.go)。
- [P13] Codex WebSocket quota 事件转 headers：[`internal/runtime/executor/helps/codex_quota.go`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/runtime/executor/helps/codex_quota.go)。
- [P14] 生命周期与重配置调用链：<https://help.router-for.me/plugin/development#lifecycle>；[`internal/pluginhost/host.go#L1035-L1049`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/internal/pluginhost/host.go#L1035-L1049)；[`sdk/cliproxy/service_config.go#L142-L180`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/sdk/cliproxy/service_config.go#L142-L180)；[`sdk/cliproxy/service_plugins.go#L112-L135`](https://github.com/router-for-me/CLIProxyAPI/blob/54946fa3dfa29c6ca7312ac141a92cdd5e413771/sdk/cliproxy/service_plugins.go#L112-L135)。
- [P15] Claude 官方 quota 响应头用途及 Retry-After 单位：<https://code.claude.com/docs/en/llm-gateway-protocol#response-headers>。
