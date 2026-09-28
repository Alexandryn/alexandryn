# Alexandryn Network Topology & LAN Architecture

This document describes the networking architecture, reverse proxy design, port isolation, hostname resolution, and failure recovery mechanisms for Alexandryn across both supported deployment models: **Docker-managed hosting** and **Desktop-managed hosting (Electron)**.

---

## 1. Network Topology Overview

Alexandryn delivers a single-entry LAN experience. Users access the application via:
```
http://alexandryn.local
```
or directly via the host's LAN IP (e.g., `http://192.168.1.50`). Internal service ports are kept private.

```
                           [ Local Area Network (LAN) ]
                                        │
           ┌────────────────────────────┴────────────────────────────┐
           ▼                                                         ▼
┌───────────────────────────────┐         ┌───────────────────────────────────────┐
│     Docker-Managed Host       │         │      Desktop-Managed Host (Electron)  │
│                               │         │                                       │
│  [Host Port 80 (or HTTP_PORT)]│         │  [Host Port 80 (or unprivileged 8080)]│
│               │               │         │                   │                   │
│         ┌─────▼─────┐         │         │             ┌─────▼─────┐             │
│         │   NGINX   │         │         │             │ Go Server │             │
│         │  Gateway  │         │         │             │  Single   │             │
│         └─────┬─────┘         │         │             │EntryPoint │             │
│               │ 172.28.0.0/24 │         │             └─────┬─────┘             │
│       ┌───────┴───────┐       │         │                   │ Unix Socket /     │
│       ▼               ▼       │         │                   │ Loopback Only     │
│ ┌───────────┐   ┌───────────┐ │         │             ┌─────▼─────┐             │
│ │  Backend  │   │ PostgreSQL│ │         │             │ PostgreSQL│             │
│ │ (Go App)  │   │  Bundled  │ │         │             │ Supervisor│             │
│ │ Port 8080 │   │ Port 5432 │ │         │             │ (Private) │             │
│ └───────────┘   └───────────┘ │         │             └───────────┘             │
│ (Private Bridge - No Host Port│         │ (Private - No LAN Exposure)           │
└───────────────────────────────┘         └───────────────────────────────────────┘
```

---

## 2. Service Listeners & Port Mapping

### Docker-Managed Hosting

| Service | Container IP / Port | Host Exposed Port | Purpose |
|---|---|---|---|
| **NGINX Reverse Proxy** | `172.28.0.2:80` | `80` (or `${HTTP_PORT}`) | User-facing entry point routing all frontend (`/`) and API (`/api/v1/`) requests. |
| **Go Backend** | `172.28.0.10:8080` | **None** (Private) | Application server, static asset server, and REST API. |
| **PostgreSQL** | `172.28.0.x:5432` | **None** (Private) | Relational database. Unreachable outside the Docker bridge network. |

- **Subnet**: Docker bridge subnet `172.28.0.0/24`.
- **Port Exposure**: Only the reverse proxy's HTTP port is published on the host. Internal backend and database ports are never published.
- **Trusted Proxy**: Backend sets `TRUSTED_PROXY_CIDRS=172.28.0.0/24`, ensuring `X-Forwarded-For` from NGINX is trusted for rate limiting and IP logging.

### Desktop-Managed Hosting (Electron)

| Service | Listener Address | Exposure | Purpose |
|---|---|---|---|
| **Go Server** | `0.0.0.0:8080` (or `80`) | LAN & Loopback | Single user-facing entry point serving embedded SPA frontend and REST API. |
| **PostgreSQL** | Loopback / Unix Domain Socket | **Local Only** | Managed by supervisor; unreachable from LAN. |
| **Electron UI** | Internal WebContents | **Local Only** | Local administration and desktop reading interface. |

---

## 3. Reverse Proxy Architecture (NGINX)

In Docker-managed hosting, NGINX acts as the unified reverse proxy:

1. **Single Entry Point**: All client traffic arrives at port 80 (or configured `HTTP_PORT`).
2. **Unified Routing**:
   - `/api/` -> Forwarded to backend with original request URI and headers.
   - `/` -> Forwarded to backend (which serves embedded static assets with client-side SPA fallback).
   - `/healthz` & `/readyz` -> Forwarded to backend for health reporting.
3. **Header Propagation**:
   - `Host`: Client requested host (`alexandryn.local` or host IP).
   - `X-Real-IP`: Client IP address.
   - `X-Forwarded-For`: Chain of client IPs.
   - `X-Forwarded-Proto`: Client scheme (`http` or `https`).
   - `Upgrade` & `Connection`: Preserved for WebSockets.
4. **Limits & Buffers**:
   - `client_max_body_size 10M` matching backend `HTTP_MAX_BODY_BYTES`.
   - Proxy buffers sized for streaming read content and large book covers.

---

## 4. LAN Hostname Resolution (`alexandryn.local` & mDNS)

Alexandryn implements an in-process RFC 6762 / 6763 Multicast DNS (mDNS) responder:

1. **Multicast Group**:
   - IPv4: `224.0.0.251:5353`
   - IPv6: `[ff02::fb]:5353`
2. **Records Advertised**:
   - `A` Record: `alexandryn.local` -> Local LAN IPv4 address(es) (e.g. `192.168.1.50`).
   - `PTR` Record: `_http._tcp.local` -> `Alexandryn._http._tcp.local`.
   - `SRV` Record: `Alexandryn._http._tcp.local` -> `alexandryn.local:<port>`.
3. **Lifecycle**:
   - **Startup**: Joins multicast group on all active non-loopback interfaces; sends gratuitous announcement.
   - **Query Handling**: Responds to standard mDNS queries for `alexandryn.local` and service discovery.
   - **Shutdown**: Sends goodbye packets with TTL 0 to immediately flush client caches.
4. **Automated Verification**:
   - Before reporting `alexandryn.local` as an active verified address, Alexandryn performs a local query check to confirm resolution.
   - If mDNS is blocked or fails, `hostnameVerified` is set to `false`, and the UI displays fallback instructions.

---

## 5. Separation of LAN from Remote Access

Alexandryn strictly separates local network hosting from remote internet exposure:

1. **Default Mode: LAN (Private)**:
   - Covers RFC 1918 private ranges (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`), IPv6 ULA (`fc00::/7`), and link-local (`169.254.0.0/16`, `fe80::/10`).
   - Plaintext HTTP is permitted on LAN.
   - Authentication is **always enforced** (JWT bearer tokens, Argon2id passwords, MFA, pairing grants). No "authentication off" mode exists.
2. **Remote Access (Public)**:
   - Requires explicit operator configuration.
   - Fail-closed: The server refuses to start on a publicly routable IP or public domain without a validated TLS certificate (`TLS_CERT_FILE`/`TLS_KEY_FILE`) or ACME domain configuration (`ACME_ENABLED=true`).
   - Public binds automatically run an HTTP->HTTPS redirect on port 80 and enforce HSTS.

---

## 6. Failure Modes & Troubleshooting

| Failure | Detection | Error / Status | Actionable Resolution |
|---|---|---|---|
| **Port Conflict** | Listener fails with `EADDRINUSE` | `PortInUse: port <p> is already in use` | Another service is using port 80 or 8080. Stop conflicting service (e.g., Apache/systemd-resolved) or set `HTTP_PORT=8080`. |
| **Port Permission Denied** | Listener fails with `EACCES` | `PermissionDenied: cannot bind port <p>` | Ports < 1024 require root or `CAP_NET_BIND_SERVICE` on Linux. Run with capabilities or use port 8080. |
| **mDNS / Multicast Blocked** | `VerifyResolution` times out | `hostnameStatus: "unverified"` | Wi-Fi router has client/AP isolation enabled, or host firewall blocks UDP 5353. Access via host LAN IP, or add `<ip> alexandryn.local` to `/etc/hosts`. |
| **No Active LAN Interface** | Interface check finds only loopback | `reachability: "loopback"` | Host machine is not connected to a network. Connect to Wi-Fi or Ethernet to enable LAN access. |
| **Proxy Failure (Docker)** | NGINX cannot reach `backend:8080` | `502 Bad Gateway` | Check `docker compose logs backend` to ensure the Go backend has completed startup and migrations. |
| **Firewall Limitation** | Incoming TCP packets dropped | Client connection timeout | Allow TCP port 80 (or HTTP port) and UDP port 5353 in host firewall (`sudo ufw allow 80/tcp`, `sudo ufw allow 5353/udp`). |

---

## 7. Manual Configuration Fallback

When multicast/mDNS is restricted by network infrastructure (e.g., corporate or guest networks with AP isolation):
1. **Direct IP Access**: Navigate directly to `http://<host-ip>` (e.g. `http://192.168.1.50`).
2. **Static Hosts Entry**:
   - **Linux / macOS**: Add to `/etc/hosts`:
     ```
     192.168.1.50 alexandryn.local
     ```
   - **Windows**: Add to `C:\Windows\System32\drivers\etc\hosts`:
     ```
     192.168.1.50 alexandryn.local
     ```
