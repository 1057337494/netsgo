# NetsGo Design Issues

本文档只记录当前仍需要单独治理的问题。已经完成或被后续 RFC 替代的文档应删除，避免把历史上下文误读成待办。

## Issue 索引

| Issue | Severity | 状态 | 说明 |
|---|---|---|---|
| [`p2p-data-transport-policy`](./issue/p2p-data-transport-policy.md) | High | Open | client_to_client 数据通道中继/P2P preferred/P2P only 策略 |
| [`secrets-management`](./issue/secrets-management.md) | High | Open for secret store | SOCKS5/HTTP auth password hash 与 API 脱敏已完成；剩余是通用 secret store、轮换和迁移 |
| [`runtime-state-active-exposed`](./issue/runtime-state-active-exposed.md) | Medium | Open | DB 使用 `active`，协议/API 使用 `exposed` 的双命名债务 |
| [`endpoint-type-extensibility`](./issue/endpoint-type-extensibility.md) | Medium | Open for CHECK relaxation | CHECK 已扩展至 `socks5_listen` / `socks5_connect_handler`；剩余是是否移除 DB enum CHECK |
| [`tunnel-resource-locks-hardening`](./issue/tunnel-resource-locks-hardening.md) | Medium | Open for DB constraints | SOCKS5/TCP 端口互斥已完成；剩余是 resource lock 的 FK/CHECK 和脏数据迁移策略 |
| [`v1-v2-api-unification`](./issue/v1-v2-api-unification.md) | Medium | Open | 统一 legacy v1 与 unified v2 API 写路径 |
| [`socks5-udp-associate`](./issue/socks5-udp-associate.md) | Medium | Open | SOCKS5 UDP ASSOCIATE 支持设计 |
| [`p2p-placeholder-cleanup`](./issue/p2p-placeholder-cleanup.md) | Low | Open | P2P 占位消息/状态的归档或实现计划 |

## 原则

1. 会触及全仓状态语义、迁移兼容、legacy API 写路径或通用安全基础设施的问题，必须独立治理。
2. 每个 issue 都应有明确验证方式，不能只记录问题不定义完成标准。
3. 完成态 issue 不保留在本索引中；相关实现证据应以代码、测试和 Git 历史为准。
