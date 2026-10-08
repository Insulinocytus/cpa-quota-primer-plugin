# 订阅用量窗口：一级资料核验

核验日期：2026-10-08。用途：grill-with-docs 质询资料，**不是已确认规格或实现承诺**。

仅查公开文档与一级源码。未读取用户凭据，未登录账号，未发送真实模型请求，未运行构建、测试或格式化。

## 结论先行

| 问题 | Codex / ChatGPT 订阅 | Claude Code / Claude Pro、Max 订阅 |
| --- | --- | --- |
| 五小时窗口由首次请求启动？ | **有明确官方依据**：上一窗口结束后，首条 Work 或 Codex 消息启动新窗口。[O1] | **未证实**：官方称五小时 session / rolling window，但本次未找到明确的 first-use 起点算法。[A1][A2][A4] |
| 真正逐请求滑动，还是首次使用锚定？ | 产品说明支持“首次使用锚定的五小时窗口”；源码也使用 rolling 一词，不能仅凭术语认定逐请求滑动。[O1][O4] | rolling 是官方原词；是否按逐请求释放用量、首次消息锚定、整点取整，均未找到公开算法。[A4] |
| 活跃窗口会被普通新请求重新计时？ | 文档只定义“上一窗口结束后的首条消息”；不支持靠窗口内普通请求重开五小时。[O1] | 未找到普通请求重开活跃窗口的官方依据。不能宣称会，也不能把“不会”当已核验后端算法。 |
| 周窗口能否一并预热？ | 存在周限制；购买 instant reset 后，下一请求明确启动新周周期，七天后重置。正常周窗口初次起点本次未证实。[O1][O2] | **不能按首次使用启动推断**：Pro / Max 周重置时间由账号指定，每周固定，不随开始使用或订阅开始改变。[A1][A2] |
| 所有订阅都有五小时窗口？ | **不是**：当前官方说明 Pro 100 / 200 / 500 无五小时限制；Plus 等适用，账号数据为准。[O1][O3] | Pro、Max 官方说明每五小时重置；不要泛化到所有 Enterprise / API 计费方式。[A1][A2] |
| 可取 reset 时间？ | 官方 app-server 提供 `account/rateLimits/read`，窗口含 `resetsAt`；有更新通知。[O5] | 官方 Claude Code statusline JSON 提供 `rate_limits.five_hour.resets_at` / `seven_day.resets_at`，但不是独立查询 API。[A4] |

这里“首次使用锚定”“逐请求滑动”是研究中的解释性分类，不是本仓库已确认领域术语。

## OpenAI：已核验事实

1. 官方原句：**“A new window starts when you send your first message in Work or Codex after the previous window ends.”** 这是“上一窗口到期后首次真实消息启动新窗口”的直接依据。其上下文明确说 five-hour window。[O1]
2. 同时有五小时与周限制时，两个窗口都须有剩余额度。新开短窗口并不恢复周额度；达到五小时限制可能发生在五小时到期之前。[O1]
3. 当前 Pro 100 / 200 / 500 没有五小时用量限制。不能只读计划名称就假定存在短周期；官方也要求查看当前 Usage 的额度与重置时间。[O1][O3]
4. Work 与 Codex 共享额度。模型、任务、上下文、reasoning、工具和速度设置影响消耗；换模型不恢复共享池额度。普通 Chat 的 GPT-6 Pro 与 Work / Codex 用量分开，不能拿 Chat 请求证明 Codex 池已启动。[O1][O3]
5. 官方协议支持 `rateLimitsByLimitId` 多桶，以 metered `limit_id` 为键；窗口给出 `usedPercent`、`windowDurationMins`、`resetsAt`。存在多桶能力不等于每个模型拥有独立窗口，更不等于请求一个模型会启动所有桶。示例中的 15 / 60 分钟不能当订阅五小时数值。[O5]
6. 一级客户端源码的 `RateLimitWindow` 注释写 “Rolling window duration, in minutes”，并定义可缺失的 `resets_at` 为 Unix 秒时间戳。源码是客户端数据模型，不是后端窗口算法。[O4]
7. `account/rateLimits/read` 是官方 app-server 账号接口；`account/rateLimits/updated` 在限制变化时通知。它不等同于公开 OpenAI Platform REST quota API，也不能凭其存在认定 CPA 插件可直接调用。[O5]
8. 普通额外 credits 在 included limits 用完后支付支持的额外使用；API key 使用单独按 API 计费。API 请求成功不是 ChatGPT 订阅窗口已启动的证据。[O2][O3]
9. **credits 与 reset 必须分开**：购买 instant reset 会立即恢复适用的五小时与周额度；首次后续 Work / Codex 请求启动新的周周期，自动周重置在该请求七天后。banked reset 也可刷新适用窗口并改变周重置日；这些是显式重置操作，不是普通新请求行为。[O1][O2]

### OpenAI：仍未知

- 无历史使用账号的第一条请求、普通周到期后首次使用的精确周起点算法。
- 失败、拒绝、取消、只收到流首包、输出零 token 的请求是否开启窗口。
- 是否有最小消费阈值、时间取整、异步计量延迟。
- 极小请求能否保证所有模型 / 所有 quota bucket 启动。
- 无窗口字段能否解释为“尚未启动”：计划不适用、数据不可用也可能无字段，不能仅凭缺失作此判断。[O4][O5]

## Anthropic：已核验事实

1. Pro 与 Max 官方均写 **“Your session-based usage limit will reset every five hours.”**，另有 all-models 周限制。[A1][A2]
2. Pro / Max 周窗口有明确不同语义：**“Weekly limits reset at a fixed time each week that is assigned to your account.”** 重置日和时间不随首次使用、订阅开始而变。[A1][A2]
3. Claude Code、Claude 网页及其他 Claude 产品入口共享订阅用量限制。登录 Console / API key 是另一计费路径；`ANTHROPIC_API_KEY` 可使 Claude Code 使用 API 而不是 Pro / Max 额度。[A3][A8]
4. 官方 statusline 文档称订阅有 rolling `five_hour` 与 weekly `seven_day`；每个窗口给 `used_percentage` 和 Unix 秒 `resets_at`。文档没有定义 rolling 的数学算法或首次消息起点。[A4]
5. `rate_limits` 对 Pro / Max 订阅仅在当前 Claude Code session 收到首个 API response 后出现；五小时、七日窗口各自可缺失，客户端会在 `resets_at` 过去后丢掉该窗口。因此“新开的 CLI session 没字段”不能证明账号窗口未活动。[A4]
6. 官方网关兼容文档要求逐响应原样转发 `anthropic-ratelimit-unified-*`；Claude Code 从成功响应读这些头以显示 claude.ai 计划额度，从 429 区分计划限制 / spend cap 与临时 throttle。这里只核验官方公开的头族，不编造具体子头 schema。[A9]
7. 官方 Usage 页面显示五小时当前 session 消耗、剩余时间，以及 all-models 和 Fable（若计划包含）的周限制。因此可能存在模型专项周限制，但没有找到“每个模型独立五小时窗口”的依据。[A5]
8. Usage credits 开启后，included limits 用完可转额外付费；官方明确 **“Usage credits don’t affect this reset timing.”** 额外付费不等于重启订阅窗口。[A6]
9. 官方 limit reset 可立即恢复指定五小时或周额度；周限制仍在原来的固定日和时间重置。与 OpenAI instant reset 会改变周重置安排不同。[A7][O2]

### Anthropic：仍未知及文档边界

- 本次官方检索没有找到“空闲账号发送第一条消息，恰好五小时后 reset”的明确规则。
- 没有找到五小时窗口整点取整、逐请求释放额度、窗口内续期 / 不续期的公开算法。
- Usage credits FAQ 另写“每五小时 once you reach it 重置”，未解释这与 session / rolling 起点关系。不能将此句改写为“达到上限才开始计时”。[A6]
- statusline 是官方客户端传出的本地状态，不是保证可在推理前独立读取的订阅 quota REST API。[A4]
- 未找到官方面向插件的独立订阅 quota 查询 API，也未确认 CPA 能无损取得 unified 响应头。
- 官方 Claude Code 仓库与公开文档未提供本次问题需要的后端计量算法证明；不能以第三方逆向、issue 用户报告替代官方结论。

## 什么证据才足以声称“窗口已启动”

官方没有定义名为“窗口启动成功”的响应码或专用事件。以下是**研究中的证据分级，不是已确认验收规格**：

- **不足**：HTTP 200、模型回复 `OK`、有输出 token、流首包、CLI 正常退出、429。它们单独不能证明目标订阅桶从未活动变为活动；429 也可能只是已耗尽窗口。
- **能证明观察到窗口状态**：对应账号 / workspace、对应目标 quota bucket 的最新官方额度状态中，有窗口额度及 reset 时间。Codex 可通过官方 `account/rateLimits/read` / 更新通知读取；Claude Code 可观察官方 statusline 数据，或由官方客户端处理的 unified 响应头状态。未来 reset 时间是否必然表示窗口已活动，仍须核验具体接口对闲置窗口的返回语义，不能仅凭字段名称保证。[O5][A4][A9]
- **要归因于这一次启动，仍需前后状态**：[INFERENCE] 请求前确认目标窗口已过期或不活动，请求后获取新短窗口 reset 时间，并排除同账号其他客户端请求及显式 reset。仅请求后的 future `resets_at` 无法排除原本已经活动。
- **不能统一写成 `resets_at = 本地发起时刻 + 5h`**：Codex 文档给起点原则，但没有给失败请求 / 取整 / 计量时刻规范；Claude 首次使用锚定本身仍未证实。
- **不把字段缺失当成功或失败**：两个 provider 的窗口可缺失；Claude 首次响应前甚至不会输出相关字段。[O4][A4]

## 对当前质询的事实约束

- “两个 provider 都必须发送最小请求才开始五小时倒计时”目前只有 Codex 适用窗口有直接依据，不能当跨 provider 已确认事实。
- 活跃窗口内发新请求不能被描述为“重置窗口”；OpenAI 的明示规则不支持它，Anthropic 未找到依据。
- 正常短窗口启动、周窗口启动、付费额外使用、显式额度 reset 是四个不同问题。
- 用户已明确目标为 Codex 五小时与周窗口、Claude 五小时窗口，且选择仅订阅额度的硬保护要求。用户提出用窗口重置时间与当前时间之差接近完整窗口长度来识别需要预热的状态；该规则的容错边界及硬保护不可用时的处理仍待最终确认。

## Claude HTTP usage 查询补充

直接客户端通过 `GET https://api.anthropic.com/api/oauth/usage` 查询，不发送模型推理请求。请求使用 OAuth Bearer token 与 `anthropic-beta: oauth-2025-04-20`。本次未找到 Anthropic 官方公开的稳定 HTTP schema；以下字段证据来自客户端自身源码及公开原始响应，不冒充官方后端契约。[客户端查询与解码](https://github.com/steipete/CodexBar/blob/a88f0819d868327680b8ccabae59b72a9cbc06f3/Sources/CodexBarCore/Providers/Claude/ClaudeOAuth/ClaudeOAuthUsageFetcher.swift)。

- `five_hour.utilization` 是已使用百分比；剩余百分比为 `100 - utilization`。
- HTTP `five_hour.resets_at` 是可解析的日期时间字符串，不是 statusline 的 Unix 秒。公开 Max 20x 响应示例为 `2026-07-21T14:10:00.268668+00:00`，兼容 RFC3339，带 UTC offset 与小数秒。[直接采集的原始响应](https://github.com/getpaseo/paseo/issues/2302)。
- 客户端读取 `extra_usage.is_enabled`，但字段缺失不能按 false 处理；读到 false 也不是针对后续请求的原子禁计费授权。
- 冷窗口、过期窗口、`utilization=0` 时的 reset 确定语义仍未找到状态明确的原始证据。optional 解码类型不能证明 null 表示未启动；未来 reset 也不能单独证明计时已经开始。
- `provider=claude` 不等于订阅身份。同 provider 可有 OAuth 与 API key；API key 不属于订阅预热集合。[CPA 账户类型](https://github.com/router-for-me/CLIProxyAPI/blob/0f96f568e4dbf6f84ad7399a74b78344c5eac7e6/sdk/pluginapi/types.go)；[执行器鉴权分支](https://github.com/router-for-me/CLIProxyAPI/blob/0f96f568e4dbf6f84ad7399a74b78344c5eac7e6/internal/runtime/executor/claude_executor.go)。

CPA `/v0/management/api-call` 可按 `auth_index` 使用 `$TOKEN$` 替换上游鉴权，但要求管理权限；外层 HTTP 200 不代表上游成功，须检查 `status_code` 并解析字符串 `body`。原生 `host.http.do` 没有自动账户 token 注入。另一条源码支持的路径是 `host.auth.get` 取得物理凭据 JSON，再经宿主 HTTP transport 查询；无物理 backing file 时不能保证可用。插件不能假定自动拥有管理权限或完整运行时 token。[管理 bridge](https://github.com/router-for-me/CLIProxyAPI/blob/0f96f568e4dbf6f84ad7399a74b78344c5eac7e6/internal/api/handlers/management/api_tools.go)；[宿主 HTTP 回调](https://github.com/router-for-me/CLIProxyAPI/blob/0f96f568e4dbf6f84ad7399a74b78344c5eac7e6/internal/pluginhost/host_callbacks.go)；[凭据回调](https://github.com/router-for-me/CLIProxyAPI/blob/0f96f568e4dbf6f84ad7399a74b78344c5eac7e6/internal/pluginhost/auth_callbacks.go)。

以上仅源码与公开资料核验，未使用账户凭据或实际调用 usage 接口。查询权限、字段完整性、冷窗口判定及最终计费保护均须单独处理，不能因为日期字符串可计算就视为已证明整个预热流程。

## Codex HTTP usage 查询补充

主线程核验 OpenAI 官方 Codex 源码，固定版本 `624b45c4dcdc43a64da0307ce59c4508ef2206e9`。后台 Codex 调查已按用户要求取消；以下不是采用未完成子代理的结论。

官方客户端使用 GET usage 查询。ChatGPT base URL 自动归一到 `/backend-api`，随后选择 `/wham/usage`；因此该路径为 `https://chatgpt.com/backend-api/wham/usage`。另一种 base URL 的路径为 `/api/codex/usage`，不能混用。[客户端 base URL 与账户 headers](https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/backend-client/src/client.rs)；[GET 查询与 URL 构造](https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/backend-client/src/client/rate_limit_resets.rs)。

原始 `primary_window` / `secondary_window` 类型可缺失或 null，窗口对象包含整数字段 `used_percent`、`limit_window_seconds`、`reset_after_seconds`、`reset_at`。官方客户端将 `reset_at` 直接映射为协议 `resets_at`，官方 app-server 文档说明后者为 Unix 秒时间戳。`used_percent` 是已使用比例，不是剩余比例。[窗口模型](https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/codex-backend-openapi-models/src/models/rate_limit_window_snapshot.rs)；[额度状态模型](https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/codex-backend-openapi-models/src/models/rate_limit_status_details.rs)；[字段映射](https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/backend-client/src/client.rs#L747-L759)；[官方协议单位](https://learn.chatgpt.com/docs/app-server#6-rate-limits-chatgpt)。

`allowed`、`limit_reached` 是额外状态字段；字段模型本身没有定义未首次使用、过期、0% 已用时 `reset_at` 的后端语义。不能把可缺失窗口当成未启动，也不能把整数 0 的特殊含义写成已有官方保证。窗口时长应以接口为准，不仅凭 primary/secondary 字段名判断具体周期。未登录、未发实际 usage 或模型请求。

## 用户给定的账户行为与判定方向

用户明确观察：查询返回的窗口重置时间始终在未来，不应以 `reset_at <= now` 作为预热条件。本轮设计以该观察为输入，不把此前提出的到期判定继续作为候选。

用户提出同时检查剩余额度百分比与剩余窗口时间：若剩余额度为 100%，且 `reset_at - now` 接近完整窗口时长，则视为需要预热；剩余时间明显短于完整时长，则视为窗口已经启动。目标仅为 Codex 五小时、Codex 周额度及 Claude 五小时三个窗口。五小时窗口时长为 5h，周窗口按对应接口报告的完整周期处理。

约 ±1 分钟是用户提出的容错方向，尚非已验证阈值。[INFERENCE] 若一个窗口刚启动不足一分钟，且整数百分比仍报告全额，同样会落入容错区间；单次查询无法以该启发式严格区分这两种状态。因此它是预热判定启发式，不是上游计费授权或唯一的窗口启动证明。

用户选择费用边界 B：需要上游强制禁止额外计费的保护，而不只是降低风险。Claude 账户侧关闭 Usage credits 有官方依据；CPA 原生 Codex OAuth 通道的同等硬保护尚未证实。尚未获得用户批准缩减 Codex 支持或将其改为允许残余计费风险。

后续范围纠正：用户指出账户侧额外付费设置属于本项目范围外，不应继续要求用户为其作插件设计决策。本项目应区分订阅身份筛选、窗口预热条件与上游计费策略；不能把项目外设置管理当作实现前提，也不能把额度预查描述为插件已经提供了请求级禁计费保证。

## 真实账户探测（2026-10-08）

用户运行一次性探测脚本（已删除），经 CPA `/v0/management/api-call` 只查询额度，未发送模型请求。一次观测，6 个 OAuth 账户（Claude 2 个、Codex 4 个）。

| 账户状态 | 观测到的原始字段 | 结论 |
|---|---|---|
| Claude 五小时窗口空闲 | `utilization: 0.0`，`resets_at: null` | Claude 未启动窗口**不报告**未来 reset 时间；“重置时间约为当前时间加 5h”的规则不适用于 Claude。 |
| Claude 五小时窗口已用 21% | `resets_at: "...T17:30:00.288669+00:00"` | 与此前公开样例一致，reset 时间落在整分钟或更粗刻度。[INFERENCE] Claude 窗口时间可能被上游取整，不能按秒级“完整 5h”判定。 |
| Codex 五小时窗口未启动 | `used_percent: 0`，`reset_after_seconds: 18000`，`reset_at - now ≈ 17999.2s` | 符合用户观察：未启动窗口报告 `reset_after_seconds` 等于完整窗口时长。 |
| Codex 五小时窗口已启动但显示 0% 已用 | `used_percent: 0`，`reset_after_seconds: 16840` | 仅凭“剩余 100%”会误判为未启动；同时检查剩余时间的规则能正确识别为已启动。 |
| Codex 五小时窗口耗尽 | `used_percent: 100`，`reset_after_seconds: 709` | 不满足预热条件。 |
| Codex 周窗口 | 4 个账户均已使用（16%–72%） | 本次没有未启动的周窗口样本；周窗口未启动时的字段形态**未验证**。 |

Codex 窗口时长可由 `limit_window_seconds` 区分：`18000` 为五小时窗口，`604800` 为周窗口。`reset_after_seconds` 由服务端计算，不受本机时钟偏差影响。

### Codex 最小请求实测（openai3，2026-10-08）

用户运行一次性预热脚本（已删除），经 `/v0/management/api-call` 固定账户发送一次 Responses 请求（模型 `gpt-5.5`，输入 `"hi"`，`stream:true`，`store:false`），上游 HTTP 200，`response.completed`，共 20 tokens。

| 时点 | `used_percent` | `reset_after_seconds` | `reset_at` |
|---|---|---|---|
| 13:10:20 空闲 | 0 | 18000 | 1791483020 |
| 13:19:19 空闲（请求前） | 0 | 18000 | 1791483559 |
| 请求后约 1–3 秒 | 0 | 18000 | 1791483562 |
| 13:20:16（请求后约 54 秒） | 0 | 17948 | 1791483563 |

结论：

- 空闲时 `reset_at` 随当前时间前移，`reset_after_seconds` 恒为 18000；请求后 `reset_at` 固定在 1791483563 附近（13:19:23 + 5h），`reset_after_seconds` 开始下降。一次最小请求**已启动** Codex 五小时窗口，窗口从请求时刻开始计时。
- 请求后 `used_percent` 仍为 0，周窗口 `used_percent` 仍为 18。仅凭已用百分比无法区分窗口是否启动，必须看剩余时间。
- 请求后约 54 秒，60 秒容错规则仍判为 `needs_warmup`。只靠该规则会在容错期内重复预热。

尚未验证：Claude 未启动窗口（`resets_at: null`）在最小请求后是否变为未来时间；Codex 周窗口未启动时的字段形态。

## 大周期重置与小周期窗口的关系（调查）

本节区分**自然周到期**与**显式额度 reset**，后者的双窗口恢复规则不能证明前者会联动。[O1][O2][A7] 未使用凭据、未查询真实账户、未发送模型请求。

### Codex（ChatGPT 订阅）

| 问题 | 结论 | 证据等级 | 来源 |
| --- | --- | --- | --- |
| 五小时剩 4h、周剩 2h；2h 后短窗口怎样？ | **未知**。官方分别定义五小时和周限制；客户端分别解码 `primary_window` / `secondary_window`，但没有给出自然周到期时如何处理仍活跃的短窗口。[INFERENCE] 短窗口继续到自身到期是符合分别计时的解释，不能排除清零、重新计时或变为未启动。 | 一级资料只证明窗口分列；联动算法未证实 | [O1][A11] |
| 周窗口是否由首请求启动？ | **购买 instant reset 后：是**，下一条 Work / Codex 请求启动新周周期，七天后重置。普通初次使用、自然周到期后是否同样等待首请求，或按既有周锚点续期：**未知**。 | 一级直接说明，仅限显式 reset 后 | [O2] |
| 管理／促销 reset 是否同时恢复两个窗口？ | 购买 instant reset、使用 **full banked reset** 均恢复适用的五小时及周额度，并改变周安排。automatic / global reset 只恢复公告指定的 eligible limits，不能泛化为每次都恢复两者；文档也没有规定短窗口计时字段是否立即进入未启动状态。 | 一级直接说明恢复范围；短窗口计时状态未知 | [O1][O2][A10] |

### Claude（Pro / Max 订阅）

| 问题 | 结论 | 证据等级 | 来源 |
| --- | --- | --- | --- |
| 五小时剩 4h、周剩 2h；2h 后短窗口怎样？ | **未知**。官方分别说明每五小时 session 与固定周重置，statusline 分列两者的 reset 时间，但没有明说周到期是否清空或重开活跃 session。[INFERENCE] 继续保持短窗口原 reset 是合理解释，不是后端保证。 | 一级资料支持不同周期；联动算法未证实 | [A1][A2][A4] |
| 周窗口是否由首请求启动？ | **不是按首次使用启动**。周重置是分配给账号的固定日和时间，不随开始使用或订阅开始而变，每周期收到完整周额度。 | 一级直接说明 | [A1][A2] |
| 管理／促销 limit reset 是否同时恢复两个窗口？ | **不保证两者一起恢复**。官方原句是 “either your five-hour session limit or your weekly usage limit”，取决于展示的 reset；周重置仍保留原固定日和时间。该说明不是所有后台管理／全局促销操作的通用契约，也未说明恢复五小时额度是否重锚计时。 | 一级直接说明产品内 limit reset；其他操作未知 | [A7] |

### 未知与单次真实账户观测建议

以下仅为 [INFERENCE] 观测方法，未执行；一次结果只能确认该账号、该次事件，不能证明所有计划的通用规则。

- **自然周到期的短窗口联动（两家）**：选短窗口已有非零消耗、其 reset 晚于周 reset 的账号；在周到期前及到期后、首次后续模型请求之前，仅查询额度，期间停止同账号所有模型请求与显式 reset。Codex 比较 `/backend-api/wham/usage` 中两个窗口的 `used_percent`、`limit_window_seconds`、`reset_after_seconds`、`reset_at`；按时长识别短／周窗口。Claude 比较 `/api/oauth/usage` 的 `five_hour` / `seven_day` 的 `utilization`、`resets_at`。短窗口消耗与原绝对 reset 均不变，支持“继续”；消耗归零但 reset 不变，支持“只恢复额度”；reset 改变则记录新锚点，不能仅凭归零判为未启动。若 Claude 变为 `resets_at:null`，或 Codex 剩余秒数恢复完整 18000 且两次间隔查询的 `reset_at` 随时间前移，则支持已有实测形态下的“未启动”。
- **Codex 普通周窗口起点**：自然周到期后保持空闲，间隔几分钟查询两次；随后由账号持有人在已知时刻作一次正常使用，再查周字段。空闲期若 `reset_after_seconds=604800` 且 `reset_at` 前移，使用后固定在请求约七天后，支持“首请求启动”；空闲期 reset 已固定且倒计时下降，则支持该次周期不等待请求。全新账号初次使用仍需另取从未使用账号作同样的一次前后观测，不能用自然到期样本替代。
- **显式 reset 的五小时计时及未明示促销范围（两家）**：仅在账号持有人本来就要应用该 reset 时，保留操作／公告覆盖范围；在操作前、操作后但首次后续使用前，查询两次并比较上述双窗口字段，再记录一次正常使用后的字段。这样可区分两者恢复、只恢复指定窗口，以及短窗口保留原锚点、立即重锚或待下一请求启动；某次公告结果不能外推其他管理 reset。

## 重置卡与预热（调查）

核验日期：2026-10-08。本节仅调查显式重置的入口、计时语义与可观测信号，不改变预热设计。回读 [O1][O2][A7][A10]，并核验固定版本 `624b45c4dcdc43a64da0307ce59c4508ef2206e9` 的 OpenAI Codex 源码。未使用用户凭据、未调用重置接口、未发送模型请求。**恢复到 100% 剩余额度，不等于窗口已变为未启动。**

### Codex / ChatGPT

| 问题 | 结论 | 证据等级 | 来源 |
| --- | --- | --- | --- |
| 如何获得、应用 instant reset？ | 适用且有资格的个人 Plus / Pro 账户可购买；Free、Go、Business、Enterprise、Edu 不适用，资格受账户和账单国家等影响。官方列出 ChatGPT web 与 Codex desktop app 的可用性；详细步骤为 desktop 的 Settings → Usage → **Buy an instant reset** → checkout，或周额度耗尽后的应用内 offer banner。只耗尽五小时不会触发该付费 banner。付款成功立即应用，不能存储或预约。文档没有给出同样完整的 web 购买步骤，也未说明 CLI 购买命令。 | 一级官方产品说明；入口细节按文档原范围记录 | [O2] |
| 如何获得、应用 banked reset？ | 属于官方向合资格账户发放的促销权益，不是购买 credits。2026-09-03 / 04 的 Astra 活动覆盖符合条件的 Plus、Pro、Business（含指定席位）；未来发放不保证。官方步骤：desktop、CLI 或 web 的 usage 设置，选 **1 reset available / Full reset**，查看到期信息后确认。源码给出 CLI 入口 **`/usage` → Redeem reset → 选择 reset → Yes, use reset**。若没有任何适用窗口需要恢复，reset 留存；成功恢复至少一个适用窗口才消耗。 | 一级官方说明及客户端源码 | [A10][A12][A13] |
| 能否用 app-server / HTTP 应用 banked reset？ | **有源码实现**：app-server JSON-RPC `account/rateLimitResetCredit/consume`，参数 `idempotencyKey`（非空；推荐 UUID）、可选 `creditId`。宿主要求 Codex backend / ChatGPT 身份。HTTP 为 `POST https://chatgpt.com/backend-api/wham/rate-limit-reset-credits/consume`，JSON 含 `redeem_request_id` 与可选 `credit_id`；列表为同前缀 `GET /wham/rate-limit-reset-credits`。另一 base URL 模式用 `/api/codex/rate-limit-reset-credits[/consume]`。这是官方客户端所用 backend 路径，**不是公开稳定 Platform API 承诺**，也不是购买 instant reset 的 checkout API。 | 一级源码直接实现；后端稳定性未承诺 | [A14][A15][A16] |
| 对五小时和周额度的精确作用？ | 购买 instant reset 立即恢复适用五小时和周额度；**full** banked reset 刷新两者并改变周重置日。不同促销 reset 的覆盖范围以 offer 为准；automatic / global reset 直接作用于公告指定的 eligible limits，不形成可存储卡。没有适用五小时限制的计划不应虚构该窗口。 | 一级直接说明；其他 offer 不外推 | [O1][O2][A10] |
| 重置后周窗口是否未启动？ | **instant reset：明确等待下一条 Work / Codex 请求**；新周周期从该请求开始，下一次自动重置在该请求七天后，不是付款七天后。**full banked reset：**官方例子把新周安排与恢复使用的日期关联，但没有 instant reset 那样精确的 first-request 算法陈述；[INFERENCE] 可能同样等待后续首次使用，仍不能当已证明字段规则。 | instant：一级明确；banked：一级例子及标注推断 | [O2][A10] |
| 重置后五小时是否未启动？`wham/usage` 会怎样？ | **未知**。官方说立即恢复额度，但没有说明显式 reset 后短窗口保留旧锚点、从重置时重锚，还是等待下一请求。[O1] 的首次消息规则明确针对上一窗口结束后的情形，不能直接外推 reset。源码窗口字段包含 `used_percent`、`limit_window_seconds`、`reset_after_seconds`、`reset_at`，没有定义 reset 后精确取值。[INFERENCE] 无并发消费且恢复完毕时应见 `used_percent=0`；两窗口的剩余秒数是否等于完整时长仍未证实。此前空闲五小时实测不是 reset 后样本。 | 一级证明字段与恢复额度；计时状态、原始值未知 | [O1][O2][A10][A14][A11]；本文件“真实账户探测” |
| 除百分比变化，能否观察 reset 已应用？ | **banked reset 有专属操作结果与卡状态**：HTTP consume 返回 `code`（`reset` / `nothing_to_reset` / `no_credit` / `already_redeemed`）及默认可缺失的 `windows_reset`；app-server 映射成 `outcome`（`reset` / `nothingToReset` / `noCredit` / `alreadyRedeemed`），不转发窗口数量。usage 可含 `rate_limit_reset_credits.available_count`；详细列表含 `id`、`reset_type`、`status`（`available` / `redeeming` / `redeemed`）、授予与到期时间。**可建模 redeemed 不保证列表一定保留已兑换记录**。可用数量下降也可能是到期，不能单独证明 reset。`account/rateLimits/updated` 仅携带 quota snapshot，不带 reset 原因；本次协议 schema 未见专用 reset notification。未找到 instant/global reset 通用事件、响应头或已应用时间戳。 | 一级源码证明 banked 操作信号；跨客户端通知及其他 reset 信号未知 | [A14][A15][A16] |

### Claude

| 问题 | 结论 | 证据等级 | 来源 |
| --- | --- | --- | --- |
| 是否有同类用户主动 reset？谁能获得？ | **有，名称为 limit reset**。官方偶尔向 eligible plans 发放；有该 offer 的账户持有人可主动应用，不必等额度耗尽。文章未给出完整合资格计划清单、领取条件、保证发放周期或购买方式，不能推定所有 Pro / Max 都能随时获得。取消／降级前未使用的 reset 会失效；若有到期日，Usage 显示。 | 一级直接说明；详细资格未知 | [A7] |
| 应用入口是什么？ | Claude web 或 Claude Desktop：**Settings > Usage → Resets → Reset for free → 再确认 Reset for free**；额度耗尽的提示也有同名按钮。官方明确按钮目前**不在 Claude Mobile、Claude Code terminal / IDE** 提供。应用后共享账户额度也恢复到 Mobile / Claude Code。未找到官方公开的 Claude Code reset 命令、app-server 方法或稳定 reset HTTP endpoint；不猜测网页内部路径。 | 一级直接说明入口与限制；程序接口未找到 | [A7] |
| 恢复哪些窗口？ | 原文为 **“either your five-hour session limit or your weekly usage limit”**：按所展示 reset 恢复指定五小时或周额度，立即回满，不能承诺一次总是双窗口恢复。使用后不可撤销，不退已付 extra usage，也不改变 usage credit balance。 | 一级直接说明 | [A7] |
| 五小时是否变为未启动？ | **未知**。文章没有给出恢复 session 后是否重锚或等待首次请求，也没规定 `/api/oauth/usage` 的 `five_hour.resets_at` 为 `null`。[INFERENCE] 成功恢复五小时额度且无并发使用时应见 `utilization=0`，但 0 不证明未启动。此前用户观察空闲窗口 `utilization=0, resets_at=null`，不是应用 reset 后的观测，不能代替。 | 额度恢复：一级；reset 后计时／null 状态：未知 | [A7]；本文件“Claude HTTP usage 查询补充”“真实账户探测” |
| 周 schedule 是否改变？ | **不改变**；原文 **“Your weekly limits still reset on their usual day and time.”**。即使周额度回满，也保留原周重置安排，不能把下一请求描述为启动新的七天周期。 | 一级直接说明 | [A7][A1][A2] |
| 有无 reset 专属字段、头或事件？ | 本次回读官方 reset、statusline、gateway 文档，**未找到公开 reset-applied 字段、事件、header 或 redeemed 列表协议**。Usage 的 Resets / offer 是可见入口，不等同于插件可订阅的完成事件；`anthropic-ratelimit-unified-*` 文档只说明额度／限制信号，没有定义显式 reset 归因字段。未公开的 web 内部接口是否返回操作结果或留存记录：未知。 | 一级资料范围内未找到；不能宣称提供方不存在内部信号 | [A7][A4][A9] |

### 未知与能裁决的一次观测

以下均为 **[INFERENCE] 观测方案，未执行**。仅由账户持有人在本来就要应用 reset 时记录；暂停同账户其他模型使用。一条“观测”可是一组操作前后记录，仅能裁决该账户、该类 reset 的本次行为，不能外推所有计划。公开后端规范仍是通用保证所需证据。

- **Codex instant / full banked 的五小时状态、full banked 周起点、reset 后原始字段**：一次完整 reset 记录，含 reset 前 quota、成功后的两次间隔至少一分钟的纯 usage 查询、随后一次持有人正常使用后的 quota；同时记操作与请求时刻。按 `limit_window_seconds` 识别两个窗口。消费归零但绝对 `reset_at` 维持旧值，支持“仅恢复额度”；固定在 reset 时刻加窗口时长、剩余秒数随空闲下降，支持“立即重锚”；空闲查询保持 `reset_after_seconds=limit_window_seconds` 且 `reset_at` 随时间前移，正常使用后才固定，支持“未启动”。instant 与 banked 需各自样本，不能互相替代。
- **Claude 恢复五小时后是否 `resets_at:null`；恢复周是否影响五小时**：一次持有人应用注明恢复范围的 limit reset，保存此前及之后、首次后续使用前的 `/api/oauth/usage` 双窗口值，再保存持有人一次正常使用后的值。五小时归零且 `resets_at:null` 支持既有空闲形态；保留原未来值支持保留锚点；变为 reset 约五小时后支持立即重锚。周 reset 值可同时核对原 schedule 未变，不能仅凭周 utilization 归零判断短窗口状态。
- **Codex banked 已兑换状态是否对其他客户端可读、列表是否保留历史**：一次以已知卡 ID 应用的 reset，保留 consume 原始结果，以及独立客户端同账户随后只读的 credits 列表和 usage；若同 ID 仍返回 `status:redeemed`，可证明该次跨客户端可读。若记录消失，只能确认此次不留存，不能以 available_count 下降替代成功结果。
- **Codex instant / global 是否有专属完成信号**：一次正常 UI reset 的经持有人脱敏的网络／客户端事件记录，包含完成 response 与后续被动通知；出现可关联操作的明确 reset ID、原因或完成时间戳才能证明该信号存在。只有额度变化不能裁决；一次未见也不能证明所有接口不存在。
- **Claude web 内部 reset HTTP 路径及完成信号**：一次持有人正常点 **Reset for free** 的脱敏网络记录，保留方法、路径、非凭据 schema、完成结果及被动事件；可裁决当次 web 的实际路径与专属结果，不能据此宣称稳定公开 API。
- **Claude 具体发放资格／Codex web instant 购买步骤**：各一份真实 offer 的脱敏画面或官方补充说明，需明确计划与领取条件，或完整 web checkout 入口。个别账号出现按钮只能证明该账号可用，不能推出全计划资格。

## 来源

以下均为一级资料；Context7 仅用于定位，关键结论回读官方原文。网页与 `main` 源码会变，此记录反映上述核验日期。

- [O1] OpenAI Help Center，Managing usage with GPT-6 Astra in Work and Codex：<https://help.openai.com/en/articles/20001516-managing-usage-with-gpt-6-astra-in-work-and-codex>
- [O2] OpenAI Help Center，Paid weekly Work and Codex rate limit resets：<https://help.openai.com/en/articles/20001507-paid-weekly-work-and-codex-rate-limit-resets>
- [O3] OpenAI ChatGPT Docs，Pricing：<https://learn.chatgpt.com/docs/pricing>
- [O4] OpenAI 官方 Codex 源码，`RateLimitSnapshot` / `RateLimitWindow`：<https://github.com/openai/codex/blob/main/codex-rs/protocol/src/protocol.rs>
- [O5] OpenAI ChatGPT Docs，Codex App Server，Authentication endpoints / Rate limits (ChatGPT)：<https://learn.chatgpt.com/docs/app-server#6-rate-limits-chatgpt>
- [A1] Claude Help Center，What is the Pro plan?：<https://support.claude.com/en/articles/8325606-what-is-the-pro-plan>
- [A2] Claude Help Center，What is the Max plan?：<https://support.claude.com/en/articles/11049741-what-is-the-max-plan>
- [A3] Claude Help Center，Use Claude Code with your Pro or Max plan：<https://support.claude.com/en/articles/11145838-use-claude-code-with-your-pro-or-max-plan>
- [A4] Claude Code Docs，Customize your status line，Available data / Rate limit usage：<https://code.claude.com/docs/en/statusline#rate-limit-usage>
- [A5] Claude Help Center，Usage limit best practices：<https://support.claude.com/en/articles/9797557-usage-limit-best-practices>
- [A6] Claude Help Center，Manage usage credits for paid Claude plans：<https://support.claude.com/en/articles/12429409-manage-usage-credits-for-paid-claude-plans>
- [A7] Claude Help Center，What is a limit reset?：<https://support.claude.com/en/articles/17007452-what-is-a-limit-reset>
- [A8] Claude Help Center，How do usage and length limits work?：<https://support.claude.com/en/articles/11647753-how-do-usage-and-length-limits-work>
- [A9] Claude Code Docs，Claude Code gateway compatibility guide，Response headers：<https://code.claude.com/docs/en/llm-gateway-protocol#response-headers>
- [A10] OpenAI Help Center，How banked Codex resets work（full banked / automatic / global reset 的区别与范围）：<https://help.openai.com/en/articles/20001498-how-banked-codex-resets-work>
- [A11] OpenAI 官方 Codex 源码，`RateLimitStatusDetails`（仅客户端双窗口数据模型，非后端联动算法；固定版本 `624b45c4dcdc43a64da0307ce59c4508ef2206e9`）：<https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/codex-backend-openapi-models/src/models/rate_limit_status_details.rs>
- [A12] OpenAI 官方 Codex 源码，CLI `SlashCommand::Usage`（`/usage` 的用途）：<https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/tui/src/slash_command.rs>
- [A13] OpenAI 官方 Codex 源码，Usage 菜单、reset 选择与确认：<https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/tui/src/chatwidget/usage.rs>
- [A14] OpenAI 官方 Codex 源码，usage / reset credits GET 与 consume POST、两种 base URL 路径：<https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/backend-client/src/client/rate_limit_resets.rs>；原始字段、consume code 与 `windows_reset`：<https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/backend-client/src/types.rs>
- [A15] OpenAI 官方 Codex 源码，app-server `account/rateLimitResetCredit/consume` 请求方法：<https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/app-server-protocol/schema/typescript/ClientRequest.ts>；账号 quota、reset 卡状态／参数／outcome 与更新通知：<https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/app-server-protocol/src/protocol/v2/account.rs>
- [A16] OpenAI 官方 Codex 源码，reset consume 鉴权、backend outcome 映射、卡详情解析：<https://github.com/openai/codex/blob/624b45c4dcdc43a64da0307ce59c4508ef2206e9/codex-rs/app-server/src/request_processors/account_processor/rate_limit_resets.rs>
