# warpstack componenets

warpstack is a Borg-style control plane for an anycast network. It schedules workloads, directs traffic flows, handles certificate issuance, manages DNS records, and handles releases across the fleet.

## cochrane

Ansible & infrastructure management. Used mostly for bootstrapping.
- WireGuard keys + tunnels
- BIRD configs, BGP sessions, DNS anycast, filters
- etcd peering over public IPs, mTLS certs
- ufw base rules, SSH, and systemd unit for corticald

## cortical
A binary called `corticald` runs on every node with the following subsystems.

### drone
- Reconcile job placement against desired state
- Workloads run with containerd

### sickbay
- Local health checks and status reporting
- Reports the status to subspace and etcd

### store
- Local bbolt cache of last known state, used when etcd is unreachable or at boot.

### certd
- Get certificates from etcd and write them to disk for transporter.

### sytemd unit
- Watchdog

## subspace
- Gossip-based memberlist across nodes
- Allows reading of liveness, withdrawal, RTT/loss sampling for topology

## etcd
- Voters on nearby PoPs
- Nodes outside the "home region" will use etcd proxy.
- Contains job state, placements, releases, link costs, certs, zone data, and audit logging
- Periodic encrypted snapshots to external S3

## Controllers
- The following services run primaries on any node elected through etcd. Limited to only being in the home-region.

### queen
- Places jobs by constraints, region presence, and affinity.
- Reschedules if subspace shows node offline

### topology
- A service to build an adaptive mesh layout
- Maintains an RTT/loss matrix from subspace data and picks mesh edges (2)

### steering
- Manages the BGP prefixes

### pki
- ACME issuer

### zonegen
- Render the DNS zone updates and reload NSD
- 

## transporter
- Edge proxy: TLS termination, caching, forwarding over the backbone

## Platform

### apiserver
- REST API for CLI and GUI

### warp (CLI)

### viewscreen (GUI)
