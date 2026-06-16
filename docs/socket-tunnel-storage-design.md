# Socket Tunnel Storage Design Notes

## Background

This note records the current design discussion for adding a reverse dynamic proxy tunnel, similar to frp's `socks5` client plugin.

Current NetsGo TCP tunnels expose a fixed target:

```text
external user -> server listen port -> client -> fixed local_ip:local_port
```

A SOCKS5-style tunnel exposes a dynamic proxy endpoint:

```text
external user -> server listen port -> client -> target chosen by each SOCKS5 request
```

The server-side ingress is still a TCP listener. The key difference is the client-side target handler: it must parse the SOCKS5 protocol and dial the requested destination dynamically instead of always dialing a fixed local service.

## Current Storage Shape

NetsGo already has a unified tunnel model:

```text
TunnelSpec = Ingress + Target + Transport
```

The current SQLite `tunnels` table already contains fields that are useful for future tunnel modes:

```text
topology
owner_client_id
ingress_location
ingress_client_id
ingress_type
ingress_config
target_location
target_client_id
target_type
target_config
transport_policy
actual_transport
p2p_state
p2p_error
p2p_session_id
```

This means the table is not fundamentally tied to only TCP, UDP, and HTTP. The main blocker is that the database currently has strict enum-like `CHECK` constraints:

```sql
CHECK (ingress_type IN ('tcp_listen', 'udp_listen', 'http_host')),
CHECK (target_type IN ('tcp_service', 'udp_service')),
CHECK (transport_policy IN ('server_relay_only', 'direct_preferred', 'direct_only')),
CHECK (actual_transport IN ('unknown', 'server_relay', 'peer_direct', 'turn_relay')),
CHECK (p2p_state IN ('idle', 'gathering', 'checking', 'connected', 'failed', 'fallback', 'closed'))
```

The transport and P2P fields are already sufficient for future choices such as:

```text
server relay only
P2P preferred
P2P only
```

So the P2P transport selection itself should not require another main-table migration later. The part that is too narrow is endpoint type extensibility, especially `target_type`.

## Migration Impact

Adding a new endpoint type will require a migration because SQLite cannot directly remove or modify a `CHECK` constraint on an existing table.

The likely migration shape is:

```text
1. Create tunnels_new with the same columns.
2. Relax the endpoint-type CHECK constraints.
3. Copy all rows from tunnels into tunnels_new.
4. Drop old tunnels.
5. Rename tunnels_new to tunnels.
6. Recreate indexes and resource-lock related indexes as needed.
```

Impact characteristics:

```text
Rows touched: every existing row in tunnels
Expected semantic change: none for existing TCP/UDP/HTTP tunnels
Traffic history: should not need to change
Registered clients: should not need to change
Runtime behavior: existing tunnels should restore with the same state
Rollback: old binary may refuse the DB after a new migration is applied
```

The main risk is not the data model itself, but a faulty table rebuild: dropped columns, incorrect copied values, missing indexes, or inconsistent resource locks.

## Recommended Storage Direction

Do not add dedicated columns for each security option or proxy mode. Use the existing structured config fields:

```text
ingress_config
target_config
```

Recommended meaning:

```text
ingress_config:
  bind_ip
  port
  domain
  allowed_source_cidrs
  http_basic_auth
  other ingress-side access controls

target_config:
  host
  port
  proxy_protocol
  username/password policy
  allowed_target_cidrs
  other target-side handler options
```

This keeps the stable table schema focused on routing identity and lifecycle, while endpoint-specific options live in JSON.

## Endpoint Type Proposal

For a SOCKS5 reverse dynamic proxy tunnel:

```text
type = "socks5" or "dynamic_proxy"
topology = "server_expose"
ingress_location = "server"
ingress_type = "tcp_listen"
target_location = "client"
target_type = "socks5_proxy"
transport_policy = "server_relay_only"
```

Example:

```json
{
  "type": "socks5",
  "ingress": {
    "location": "server",
    "type": "tcp_listen",
    "config": {
      "bind_ip": "0.0.0.0",
      "port": 6005,
      "allowed_source_cidrs": ["203.0.113.0/24"]
    }
  },
  "target": {
    "location": "client",
    "client_id": "client-1",
    "type": "socks5_proxy",
    "config": {
      "username": "user",
      "password": "secret",
      "allowed_target_cidrs": ["10.0.0.0/8", "192.168.0.0/16"]
    }
  },
  "transport_policy": "server_relay_only"
}
```

The exact public type name should be decided carefully. `socket` is broad and can be confused with TCP sockets, WebSocket, or Unix sockets. `socks5` is more precise if the first implementation is specifically SOCKS5.

## Security Configuration Placement

Security options should be modeled by enforcement location.

Server ingress-side options:

```text
allowed_source_cidrs
bind_ip
HTTP Basic auth for HTTP tunnels
connection limits
rate limits
```

Client target-side options:

```text
SOCKS5 username/password
allowed_target_cidrs
target address denylist
per-target dial timeout
```

This distinction matters because the server can only enforce what it sees at the ingress. SOCKS5 destination addresses are visible only after the SOCKS5 handshake, which runs on the client side if the SOCKS5 handler is implemented there.

## What To Relax

The safest long-term direction is to avoid database enum constraints for endpoint types:

```text
ingress_type
target_type
```

These should be validated in Go code and through client capability checks instead.

Keep structural constraints that protect the shape of the model:

```text
topology
ingress_location
target_location
desired_state
runtime_state
```

Transport and P2P values may remain constrained because their future states are already represented, but this should be reviewed before migration. If we expect more transport implementations, these may also be better enforced in code rather than SQLite.

## Compatibility Strategy

Existing tunnels must survive migration unchanged.

For existing rows:

```text
tcp  -> ingress_type=tcp_listen, target_type=tcp_service
udp  -> ingress_type=udp_listen, target_type=udp_service
http -> ingress_type=http_host, target_type=tcp_service
```

The migration should preserve:

```text
id
name
client_id
type
local_ip
local_port
remote_port
domain
ingress_config
target_config
transport_policy
actual_transport
p2p_state
desired_state
runtime_state
error
created_at
updated_at
```

The old `local_ip`, `local_port`, `remote_port`, and `domain` columns should remain for compatibility with existing API views and legacy conversion paths. New dynamic proxy tunnels can set `local_ip=''` and `local_port=0`, while their actual behavior is defined by `target_config`.

## Validation Checklist

Migration tests should verify:

```text
tunnels row count is unchanged
existing tunnel ids are unchanged
TCP tunnel fields are unchanged
UDP tunnel fields are unchanged
HTTP tunnel fields are unchanged
client_to_client server-relay tunnels are unchanged
resource locks are preserved or correctly rebuilt
traffic_buckets are not modified unexpectedly
old runtime_state mappings remain compatible
new socks5/dynamic target_type can be inserted and read back
```

Runtime tests should verify:

```text
existing TCP tunnel still restores
existing UDP tunnel still restores
existing HTTP tunnel still routes by domain
new dynamic proxy tunnel can be created
new dynamic proxy tunnel can be stopped/resumed/deleted
source CIDR restrictions are enforced at server ingress
SOCKS5 auth is enforced by the client-side handler
target CIDR restrictions are enforced before client dial
```

## Open Questions

1. Should the public tunnel type be `socks5`, `dynamic_proxy`, or `socket`?
2. Should HTTP Basic auth be represented as a generic ingress auth object shared by HTTP and future TCP-like ingress modes?
3. Should SOCKS5 credentials be stored directly in `target_config`, or should they be referenced by secret ID in a separate secrets table later?
4. Should endpoint type constraints be completely removed from SQLite, or only expanded to include known near-term types?
5. Should transport-related constraints also move from SQLite to Go validation, or are the current P2P transport enum values stable enough?

