## Project

CLIProxyAPI 原生 cgo 插件 `cpa-quota-primer`：按 cron 对存在未启动窗口、且无耗尽窗口的 Codex / Claude OAuth 账户发送预热请求。行为与配置见 `README.md`。

- 根包 `primer`：配置解析（`config.go`）、窗口判定（`judge.go`）、预热轮次（`primer.go`）。纯 Go，测试不联网、不用真实账户。
- `cmd/plugin`：C ABI 外壳、宿主回调（`host.model.execute`、`host.log`）与 cron 调度，需要 cgo。
- 验证：`go vet ./... && go test -race ./...`。构建：`go build -buildmode=c-shared -o cpa-quota-primer.<dll|so|dylib> ./cmd/plugin`。

## Agent skills

### Issue tracker

工单与规格使用 GitHub Issues；操作前读取 `docs/agents/issue-tracker.md`。

### Triage labels

使用五个默认 triage 标签；分诊前读取 `docs/agents/triage-labels.md`。

### Domain docs

采用 single-context；探索代码前读取 `docs/agents/domain.md`。
