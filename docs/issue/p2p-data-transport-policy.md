# P2P data transport policy

## Status

Open

## Severity

High

## Why it matters

client_to_client 数据通道未来应支持 `server_relay_only`、`direct_preferred`、`direct_only`。控制通道仍必须经 server，下述策略只针对数据通道。P2P 是 transport/data-channel selector 能力，不是 TCP/UDP/HTTP/SOCKS5 或未来 endpoint type 的类型内能力。

## Current evidence

存储模型和 DTO 已有 `transport_policy`、`actual_transport`、`p2p_state` 等字段，`DataStreamHeader` 也已有 `transport` 字段，但 P2P 发送/接收实现尚未形成完整闭环。当前 `/api/tunnels` 创建阶段会拒绝非 `server_relay_only`，错误码为 `direct_transport_unavailable`。

主要代码位置：

- `pkg/protocol/types.go` 的 transport/P2P 字段
- `internal/server/migrations/005_unified_tunnel_storage.sql`
- `internal/server/client_relay.go`
- `internal/server/unified_tunnel_reconcile.go`
- `pkg/protocol/stream_header.go`
- `internal/server/data.go`
- `internal/client/client.go`
- client 数据通道和 stream 打开路径

## Recommended direction

先抽象统一 data-channel/transport selector：上层 tunnel runtime 只表达从 ingress 到 target 打开数据流，底层根据 `transport_policy` 选择 `server_relay` 或 `peer_direct`。

当前 `/ws/data + yamux` 路径应作为 `server_relay` transport 被封装；P2P 实现应新增 `peer_direct` transport，并在同一套 selector/state machine 中处理候选收集、握手、fallback、direct_only 失败语义、TURN/relay 统计与 UI 展示。

## Why separate

当前实现仍限定 `server_relay_only`。P2P 是跨 server/client/protocol 的大设计，不能作为 endpoint 类型附带修改，也不能复制到每个隧道类型里。TCP/UDP/HTTP/SOCKS5 以及未来 endpoint type 应复用同一套数据通道选择逻辑。

## Validation needed

- relay only / preferred / only 三种策略行为明确。
- direct 失败时 fallback 或 error 符合策略。
- 控制通道始终经 server。
- 数据面统计能区分 transport。
- 同一套 transport selector 能服务 TCP/UDP/HTTP/SOCKS5 与未来 endpoint type，不能为每个类型重复实现 P2P。
