# Container Orchestrator (con)

A lightweight multi-container orchestrator for Linux, built from scratch using Go.

## Overview

`con` is a container runtime that manages isolated Linux containers using kernel namespaces, cgroups, and virtual networking. It provides IPAM (IP Address Management), DNS resolution between containers, and lifecycle management commands.

## Requirements

### System Requirements

| Requirement | Version | Purpose |
|-------------|---------|---------|
| Linux Kernel | 5.x+ | Namespaces, cgroups2, veth |
| Go | 1.21+ | Build toolchain |
| crane | latest | Pull container images |
| iproute2 | 6.x+ | Network namespace management |
| nftables/firewalld | - | Bridge firewall rules |
| Root access | - | All operations require root |

### Runtime Dependencies

```
- /usr/bin/crane     # Pull container rootfs from registries
- /usr/bin/ip        # Network namespace and veth management
- /usr/bin/nsenter   # Enter container namespaces
- /usr/bin/unshare   # Create new namespaces
- /usr/bin/nft       # Firewall rules (nftables)
```

### Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-01 | Create containers from OCI images | High |
| FR-02 | Destroy containers | High |
| FR-03 | List running containers | High |
| FR-04 | Execute commands in containers | High |
| FR-05 | Allocate unique IPs to containers (IPAM) | High |
| FR-06 | DNS resolution between containers | Medium |
| FR-07 | Name containers with random names | Medium |
| FR-08 | Persist container state across reboots | Medium |
| FR-09 | Clean up orphaned resources | Low |

### Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR-01 | Single static binary deployment |
| NFR-02 | All state stored in JSON files (no database) |
| NFR-03 | Graceful error handling and cleanup |
| NFR-04 | Sub-second container creation |
| NFR-05 | Clear error messages with remediation hints |

---

## Features

### Core Features

| Feature | Command | Description |
|---------|---------|-------------|
| Create | `con create [image]` | Create and start a new container |
| Destroy | `con destroy <name>` | Stop and remove a container |
| List | `con list` | Display all containers |
| Exec | `con exec <name> <cmd>` | Run command in container |

### Networking Features

| Feature | Description |
|---------|-------------|
| Bridge Network | Shared `br0` bridge for all containers |
| IPAM | Automatic IP allocation from `10.88.0.0/24` |
| DNS | Containers resolve each other by name via `/etc/hosts` |
| NAT | Containers can reach external networks |

### Container Isolation

| Namespace | Purpose |
|-----------|---------|
| PID | Isolated process tree |
| Network | Isolated network stack |
| Mount | Isolated filesystem |
| UTS | Isolated hostname |
| Cgroup | Isolated cgroup hierarchy |

---

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                          con CLI                                │
│                    (cmd/con/main.go)                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Command Handlers                           │
│         (internal/cmd/create.go, destroy.go, ...)              │
└─────────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│   Container   │   │    Network    │   │     State     │
│   Manager     │   │    Manager    │   │   Manager     │
└───────────────┘   └───────────────┘   └───────────────┘
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│     IPAM      │   │     DNS       │   │    JSON       │
│   Allocator   │   │   Manager     │   │   Storage     │
└───────────────┘   └───────────────┘   └───────────────┘
```

### Directory Structure

```
/opt/containers/
├── ipam.json                    # IP allocation state
├── containers.json              # Container registry
└── <container-id>/
    ├── container.json           # Container metadata
    ├── rootfs/                  # Container filesystem
    ├── hostname                 # /etc/hostname content
    ├── hosts                    # /etc/hosts content
    └── resolv.conf              # DNS resolver config

/var/run/netns/
└── ctr-<id>                     # Network namespace reference
```

### Component Overview

#### 1. Container Manager (`internal/container`)

**Responsibilities:**
- Create container rootfs from OCI images
- Set up namespaces (PID, Mount, UTS, Network)
- Execute container init process
- Track container PID
- Clean up container resources

**Key Functions:**
```go
func Create(ctx context.Context, opts CreateOptions) (*Container, error)
func Destroy(ctx context.Context, id string) error
func List(ctx context.Context) ([]*Container, error)
func Exec(ctx context.Context, id string, cmd []string) error
```

#### 2. Network Manager (`internal/network`)

**Responsibilities:**
- Create/manage bridge interface
- Create veth pairs
- Attach veth to bridge and container namespace
- Configure container networking
- Set up NAT for external access

**Key Functions:**
```go
func EnsureBridge(name string, subnet *net.IPNet) error
func CreateVeth(hostEnd, containerEnd string) error
func AttachToBridge(veth, bridge string) error
func MoveToNetns(veth, netns string) error
func ConfigureContainerNet(netns, ip, gateway string) error
```

#### 3. IPAM (`internal/ipam`)

**Responsibilities:**
- Manage IP address pool
- Allocate unique IPs
- Free IPs on container destruction
- Persist allocation state

**State File (`/opt/containers/ipam.json`):**
```json
{
  "subnet": "10.88.0.0/24",
  "gateway": "10.88.0.1",
  "allocated": {
    "10.88.0.2": "a1b2c3d4",
    "10.88.0.3": "e5f6g7h8"
  },
  "reserved": ["10.88.0.1"]
}
```

**Key Functions:**
```go
func (ipam *IPAM) Allocate(containerID string) (net.IP, error)
func (ipam *IPAM) Release(ip net.IP) error
func (ipam *IPAM) Load(path string) error
func (ipam *IPAM) Save(path string) error
```

#### 4. DNS Manager (`internal/dns`)

**Responsibilities:**
- Generate `/etc/hosts` content for all containers
- Update hosts files when containers change
- Support name resolution between containers

**Approach:**
Each container gets a hosts file mounted at `/etc/hosts`:
```bash
127.0.0.1   localhost
::1         localhost
10.88.0.2   happy_rabbit
10.88.0.3   clever_fox
10.88.0.4   brave_lion
```

**Key Functions:**
```go
func (dns *DNS) GenerateHosts(containers []*Container) string
func (dns *DNS) UpdateAll(containers []*Container) error
func (dns *DNS) UpdateContainer(c *Container, hosts string) error
```

#### 5. State Manager (`internal/state`)

**Responsibilities:**
- Load/save container state
- Load/save IPAM state
- Atomic writes with temp file + rename

**Container State (`/opt/containers/<id>/container.json`):**
```json
{
  "id": "a1b2c3d4",
  "name": "happy_rabbit",
  "image": "debian:stable-slim",
  "ip": "10.88.0.2",
  "pid": 12345,
  "netns": "ctr-a1b2c3d4",
  "veth_host": "veth-a1b2c3d4",
  "veth_container": "vctr-a1b2c3d4",
  "created_at": "2024-01-15T10:30:00Z",
  "status": "running"
}
```

**Key Functions:**
```go
func (s *Store) LoadContainer(id string) (*Container, error)
func (s *Store) SaveContainer(c *Container) error
func (s *Store) DeleteContainer(id string) error
func (s *Store) ListContainers() ([]*Container, error)
```

#### 6. Name Generator (`pkg/names`)

**Responsibilities:**
- Generate memorable container names
- Ensure uniqueness

**Pattern:** `<adjective>_<animal>`
Examples: `happy_rabbit`, `clever_fox`, `brave_lion`, `swift_eagle`

---

## Data Flow

### Container Creation Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        con create debian:stable-slim                │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  1. Generate container ID (8 hex chars)                             │
│  2. Generate random name (adjective_animal)                         │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  3. IPAM: Allocate IP address (e.g., 10.88.0.2)                     │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  4. Create container directory: /opt/containers/<id>/               │
│  5. Pull rootfs: crane export debian:stable-slim | tar -x           │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  6. Network: Ensure bridge exists                                   │
│  7. Network: Create network namespace (ctr-<id>)                    │
│  8. Network: Create veth pair (veth-<id>, vctr-<id>)                │
│  9. Network: Attach veth to bridge, move vctr to netns              │
│  10. Network: Configure container IP and routes                     │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  11. DNS: Generate /etc/hosts for ALL containers                    │
│  12. DNS: Update hosts file for ALL containers                      │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  13. Namespaces: Launch container in new namespaces                 │
│      - PID, Mount, UTS (via unshare)                                │
│      - Network (via ip netns exec)                                  │
│  14. Filesystem: pivot_root into rootfs                             │
│  15. Process: Exec /bin/bash                                        │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  16. State: Save container.json                                     │
│  17. Output: Display container name, ID, IP                         │
└─────────────────────────────────────────────────────────────────────┘
```

### Container Destruction Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        con destroy happy_rabbit                     │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  1. Lookup container by name or ID                                  │
│  2. Load container state                                            │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  3. Kill container process (SIGKILL to PID)                         │
│  4. Delete veth interface                                           │
│  5. Delete network namespace                                        │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  6. IPAM: Release IP address                                        │
│  7. Remove container directory                                      │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  8. DNS: Regenerate /etc/hosts for remaining containers             │
│  9. Output: Display confirmation                                    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Go Project Structure

```
con/
├── cmd/
│   └── con/
│       └── main.go                 # Entry point, CLI setup
├── internal/
│   ├── cmd/
│   │   ├── root.go                 # Root command (cobra)
│   │   ├── create.go               # con create
│   │   ├── destroy.go              # con destroy
│   │   ├── list.go                 # con list
│   │   └── exec.go                 # con exec
│   ├── container/
│   │   ├── container.go            # Container type and operations
│   │   ├── create.go               # Container creation logic
│   │   ├── destroy.go              # Container destruction logic
│   │   └── exec.go                 # Exec into container
│   ├── network/
│   │   ├── bridge.go               # Bridge management
│   │   ├── namespace.go            # Network namespace ops
│   │   └── veth.go                 # Veth pair management
│   ├── ipam/
│   │   └── ipam.go                 # IP address management
│   ├── dns/
│   │   └── hosts.go                # DNS hosts file management
│   ├── state/
│   │   └── state.go                # State persistence
│   └── rootfs/
│       └── rootfs.go               # Rootfs extraction and setup
├── pkg/
│   └── names/
│       ├── generator.go            # Random name generation
│       └── data.go                 # Adjectives and animals lists
├── go.mod
├── go.sum
├── Makefile
└── ARCHITECTURE.md                 # This file
```

---

## External Commands Used

| Command | Usage | Alternative |
|---------|-------|-------------|
| `crane export` | Pull container image | skopeo, docker export |
| `ip link` | Create/delete interfaces | netlink library |
| `ip netns` | Manage network namespaces | netlink library |
| `ip addr` | Configure IP addresses | netlink library |
| `ip route` | Configure routes | netlink library |
| `nsenter` | Enter namespaces | syscall package |
| `unshare` | Create namespaces | syscall package |
| `pivot_root` | Change root filesystem | syscall package |
| `mount` | Mount filesystems | syscall package |
| `nft` | Firewall rules | netfilter library |

**Future improvement:** Replace shell commands with native Go libraries (netlink, syscall) for better performance and error handling.

---

## Error Handling Strategy

### Error Categories

| Category | Example | Recovery |
|----------|---------|----------|
| User Error | Invalid container name | Show usage, exit 1 |
| Resource Error | IP pool exhausted | Suggest cleanup, exit 2 |
| System Error | Bridge creation failed | Check prerequisites, exit 3 |
| State Error | Corrupted state file | Suggest reset, exit 4 |

### Cleanup on Failure

When container creation fails partway:
1. Remove veth interface if created
2. Remove network namespace if created
3. Release allocated IP
4. Remove container directory
5. Log error with context

---

## Security Considerations

| Aspect | Implementation |
|--------|----------------|
| Root required | Check EUID=0 at startup |
| File permissions | Container dirs owned by root, mode 0700 |
| Network isolation | Each container in own network namespace |
| Filesystem isolation | pivot_root with read-only /proc, /sys |
| Resource masking | Mask /proc/kcore, /sys/firmware, etc. |
| Device nodes | Minimal set: null, zero, random, urandom, tty |

---

## Future Enhancements

| Feature | Description | Priority |
|---------|-------------|----------|
| Resource limits | CPU, memory limits via cgroups | High |
| Health checks | Container health monitoring | Medium |
| Image caching | Cache pulled images locally | Medium |
| Volume mounts | Host directory mounting | Medium |
| Port forwarding | Expose container ports | Medium |
| Logs | Container stdout/stderr capture | Low |
| Multiple networks | Multiple bridges per container | Low |
| Container images | Build images from Dockerfile | Low |
