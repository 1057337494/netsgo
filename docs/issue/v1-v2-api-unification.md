# v1/v2 API 写路径统一

## Status

Open

## Severity

Medium

## Why it matters

legacy v1 `/api/clients/{id}/tunnels` 与 unified v2 `/api/tunnels` 可能以不同状态机写入同一存储模型，长期维护风险高。

## Current evidence

两套 API 使用不同请求结构和创建/下发顺序。当前前端创建/更新 tunnel 走 unified `/api/tunnels`，但 legacy v1 `/api/clients/{id}/tunnels` 仍存在并由 `ProxyNewRequest` 驱动。

主要代码位置：

- legacy v1 client tunnel API：`internal/server/admin_api.go` 及相关 tunnel manager 路径
- unified v2 API：`internal/server/unified_tunnel_api.go`
- provisioning/ack 路径：`internal/server/tunnel_ready.go`

## Recommended direction

让 v1 内部转译到 v2 的统一服务层，或者明确 v1 只作为兼容入口并限制支持范围。

SOCKS5 和后续 endpoint 类型默认只支持 v2 `/api/tunnels` 创建；v1 若收到不可表达的类型，应返回清晰错误，而不是扩展 `ProxyNewRequest`。

## Why separate

统一 v1/v2 会触及旧 API、管理面兼容、provisioning 顺序、offline managed tunnel 和 upgrade/rollback 行为，必须独立治理。

## Validation needed

- v1/v2 创建同类 tunnel 结果一致。
- 错误码一致。
- revision 与 provisioning 行为一致。
