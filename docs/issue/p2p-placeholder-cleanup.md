# P2P placeholder 清理

## Status

Open

## Severity

Low

## Why it matters

P2P 相关消息、字段或状态如果只是占位，容易让读代码者误判功能已实现。

## Current evidence

存储、协议和前端模型中已有 P2P 字段、消息类型和展示标签，但完整 P2P 数据面尚未实现。UI 文案目前把 direct/P2P 标为 unavailable，创建阶段也拒绝 direct policy。

主要代码位置：

- `pkg/protocol/types.go`
- `internal/server/migrations/005_unified_tunnel_storage.sql`
- `internal/server/unified_tunnel_reconcile.go`
- 前端 tunnel 状态展示相关代码

## Recommended direction

要么补齐 P2P 设计与实现路线，要么在代码/文档中明确标注 future-only，并清理无用占位。

## Why separate

P2P 清理与当前 server-relay 数据路径正确性无关。它应该跟 P2P transport policy 设计一起处理，或作为单独的 future-only 标注清理。

## Validation needed

- 文档与 UI 不暗示 P2P 已可用。
- 未实现路径不可被用户配置触发。
- future-only 字段有测试保护或清晰注释。
