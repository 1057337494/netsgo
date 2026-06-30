# SOCKS5 UDP ASSOCIATE

## Status

Open

## Severity

Medium

## Why it matters

RFC 1928 定义了 UDP ASSOCIATE，但它不是 CONNECT 的简单扩展，需要 UDP relay 地址、TCP control connection 生命周期、NAT 和超时语义。

## Current evidence

当前实现只支持 CONNECT。`internal/socks5wire` 能识别 UDP ASSOCIATE command，但会返回 command unsupported；没有 UDP relay 地址分配、UDP 包转发、association 生命周期或统计。

相关代码范围未来会涉及：

- server SOCKS5 listener/runtime
- client UDP handler 与 UDP association 管理
- `pkg/mux` UDP frame 相关代码
- traffic accounting
- frontend/API 配置模型

## Recommended direction

单独设计 SOCKS5 UDP：明确 UDP relay 监听位置、帧格式、会话超时、权限策略和流量统计。

## Why separate

UDP ASSOCIATE 不是 CONNECT 的小扩展。它需要独立的数据面生命周期、NAT/超时语义和资源限制，不能混进 CONNECT runtime 维护。

## Validation needed

- UDP ASSOCIATE RFC reply 正确。
- UDP 包转发和关联 TCP 连接生命周期一致。
- NAT/超时/资源限制可控。
