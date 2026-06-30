# runtime_state active/exposed 双命名

## Status

Open

## Severity

Medium

## Why it matters

legacy `ProxyConfig` 使用 `exposed`，unified `TunnelSpec` 与 DB 使用 `active`。两套状态名长期共存会让 SQL、事件、API DTO、前端展示和测试继续背负翻译成本。

## Current evidence

- SQLite CHECK 只允许 `active`，不允许 `exposed`。
- Go 协议同时存在 `TunnelRuntimeStateActive = "active"` 和 `ProxyRuntimeStateExposed = "exposed"`。
- 存储层存在 `storageRuntimeStateFromProtocol` / `protocolRuntimeStateFromStorage` 翻译函数。
- unified API 会把 legacy `exposed` 投影成 `active`，也会把 unified `active` 转回 legacy `ProxyConfig` 的 `exposed`。
- 前端类型和展示逻辑仍兼容 `exposed` 与 `active`。

主要代码位置：

- `internal/server/migrations/005_unified_tunnel_storage.sql`
- `pkg/protocol/types.go`
- `internal/server/store.go`
- `web/src/lib/tunnel-model.ts`

## Recommended direction

单独决定最终命名。倾向统一到 `active`，因为 unified `TunnelSpec`、DB、E2E 脚本和新 API 已经以 `active` 为当前形态；如果要统一到 `exposed`，需要反向迁移 DB CHECK、E2E、unified DTO 和前端类型。无论选哪边，都必须保留旧值读取兼容窗口。

## Validation needed

- legacy `ProxyConfig` 与 unified `TunnelSpec` 的状态命名边界明确。
- 所有写路径只写最终选定状态名。
- API/事件流/前端只暴露最终选定状态名，或仅在兼容边界转换。
- 旧 DB 与旧 API 返回值在兼容窗口内可读取。
- TCP/UDP/HTTP 隧道恢复行为不变。
