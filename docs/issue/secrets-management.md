# Secret store 后续治理

## Status

Open for secret store

## Severity

High

## Why it matters

SOCKS5 username/password 与 HTTP Basic auth 的本期明文落库问题已经处理，但 endpoint config 仍缺少统一 secret 引用模型。后续如果继续把不同凭据散落在 JSON config、管理员配置或未来 endpoint 配置里，轮换、删除、迁移和审计都会变成局部特判。

## Current evidence

已完成：

- SOCKS5 与 HTTP Basic auth password 通过 `internal/credential` 的 Argon2id hash 保存。
- API 提交 `password_hash` 会被拒绝，hash 是 server-owned 字段。
- tunnel 详情、列表、事件可见的 endpoint config 会移除 `password` / `password_hash`。
- server -> target client provisioning 不携带 ingress auth password hash；只有 ingress 角色需要认证配置。

仍未完成：

- 没有统一 `secrets` table 或 secret store。
- endpoint config 仍直接保存 `password_hash`，不是保存 secret ID 引用。
- 没有 secret rotation / deletion / migration 的通用流程。
- 没有 encryption-at-rest 设计。

主要代码位置：

- endpoint config 类型与 JSON 字段：`pkg/protocol/types.go`
- unified API config decode/encode：`internal/server/unified_tunnel_api.go`
- 存储 JSON config：`internal/server/store.go`
- password hash/verify：`internal/credential/password.go`、`internal/server/socks5_config.go`
- 前端表单和模型：`web/src/lib/tunnel-model.ts`、`web/src/components/custom/tunnel/`

## Recommended direction

单独设计通用 secret infrastructure：

- `secrets` table 或等价 secret store；
- endpoint config 只保存 secret ID / version reference；
- secret rotation、删除、引用计数或 orphan 检测；
- 旧 endpoint config 中 `password_hash` 的迁移策略；
- HTTP Basic、SOCKS5 auth、未来 API/token/第三方凭据复用同一模型。

## Validation needed

- API 不能返回 secret 值、hash 值或可直接用于认证的材料。
- endpoint 创建/更新通过 secret reference 工作。
- secret rotation 后既有 tunnel 可继续运行或按设计重新 provision。
- secret 删除时能阻止仍被引用的 secret 被误删，或有明确 cascade/error 语义。
- 旧 `password_hash` config 能迁移到新模型，失败时不丢配置。
