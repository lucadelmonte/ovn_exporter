# Open Virtual Network (OVN) Exporter

[![GitHub Release](https://img.shields.io/github/v/release/Liquescent-Development/ovn_exporter)](https://github.com/Liquescent-Development/ovn_exporter/releases)
[![Go Version](https://img.shields.io/badge/go-1.24-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Export Open Virtual Network (OVN) data to Prometheus.

> **Note**: This is a modernized fork of the original [greenpau/ovn_exporter](https://github.com/greenpau/ovn_exporter) which is now archived. This version includes security improvements, updated dependencies, and compatibility with Go 1.24+.

## Features

This exporter collects metrics from the following OVN components:
* OVN `northd` service
* OVS `vswitchd` service
* `OVN Northbound` database
* `OVN Southbound` database
* `Open_vSwitch` database

## What's New in v2.6.0

### New Features
- **Port binding distribution metrics.** Two new metrics from the OVN Southbound `Port_Binding` table:
  - `ovn_port_binding_count{chassis, chassis_name, type}` reports the per-chassis distribution of every port binding type. Unscheduled bindings get `chassis="unbound"`; plain tenant VIF ports (empty OVN type) get `type="vif"`.
  - `ovn_gateway_port_info{chassis, chassis_name, type, logical_port, datapath, router_name, ...}` is a drill-down for chassis-pinned gateway ports (`type=chassisredirect` or `type=l3gateway`). `router_name` is extracted from `external_ids[neutron:router_name]` when present.
- **Dashboard.** OVN System Overview gains a stacked timeseries of chassisredirect distribution by chassis, and a table mapping gateway ports to routers and chassis.

### Dependency
- Bumps `lucadelmonte/ovsdb` to v1.1.2 for the new `GetPortBindings()` function.

## What's New in v2.5.1

### Bug Fixes
- **Fix request counter accounting.** `GetAppClusteringInfo()` used `IncrementRequestCounter()` on both branches instead of the success/error variants, so `ovn_requests_total` drifted above `ovn_successful_requests_total + ovn_failed_requests_total`. Dashboards computing `ovn_successful_requests_total / ovn_requests_total` saw an artificial ceiling below 100% even when nothing was actually failing.

## What's New in v2.5.0

### Breaking Changes
- **OVS dependency removed.** The exporter no longer connects to the Open vSwitch database. `ovn_info` labels now reflect OVN-side sources only:
  - **Dropped:** `rundir`, `ovs_version`, `db_version`
  - **Added:** `nb_schema_version`, `sb_schema_version` (from the OVN Northbound and Southbound schemas)
  - `system_id` and `hostname` come from `os.Hostname()`; `system_type`/`system_version` come from `/etc/os-release`
- **`ovn_pid` metric removed.** The `/proc/<pid>` probe produced wrong data inside a container PID namespace (Kolla, etc.) and bumped the failure counter on hosts where `ovn-northd` ran inside a container. Component liveness is now probed through each daemon's JSON-RPC control socket, so `ovn_failed_requests_total` no longer climbs from that cause.
- **Flags removed (startup now errors on them):** `-database.vswitch.*`, `-service.vswitchd.*`, `-system.run.dir`. The `*-monitoring` component variants are gone with the PID walk that produced them.
- **New flag:** `-service.ovn.northd.socket.control` (defaults to `unix:/run/openvswitch/ovn-northd.ctl`). Point this at the real northd control socket on your deployment (Kolla uses `/run/ovn/ovn-northd.ctl`).

## What's New in v2.2.0

### New Features
- **Logical Router Metrics**: Added comprehensive metrics for OVN logical routers
  - `ovn_logical_router_info` - Basic router information
  - `ovn_logical_router_ports` - Number of router ports
  - `ovn_logical_router_static_routes` - Static routes count
  - `ovn_logical_router_nat_rules` - NAT rules count
  - `ovn_logical_router_load_balancers` - Load balancers count
  - `ovn_logical_router_policies` - Routing policies count
  - `ovn_logical_router_external_id` - External IDs for routers
- Complete L3 network topology visibility alongside existing L2 metrics

## What's New in v2.1.2

### New Features
- Added `ovn_requests_total` metric to track total number of requests to OVN stack
- Added `ovn_successful_requests_total` metric to track successful requests to OVN stack
- Improved request tracking and monitoring capabilities for better observability

## What's New in v2.1.1

### Breaking Changes
- **Go 1.24+ required** (upgraded from Go 1.20)
- **Module path changed** to `github.com/Liquescent-Development/ovn_exporter`
- **Logging system migrated** from deprecated `promlog` to `promslog/slog`
- **Metric names standardized** to follow Prometheus conventions:
  - `ovn_failed_req_count` → `ovn_failed_requests_total`
  - `ovn_log_file_size` → `ovn_log_file_size_bytes`
  - `ovn_db_file_size` → `ovn_db_file_size_bytes`
  - `ovn_log_event_count` → `ovn_log_events_total`
  - `ovn_memory_usage` → `ovn_memory_usage_bytes`
  - `ovn_next_poll` → `ovn_next_poll_timestamp_seconds`
- **Security improvements**:
  - Removed insecure `pprof` debug endpoints
  - Added HTTP server timeouts and security headers
  - Updated all dependencies to latest versions

### Improvements
- Multi-architecture support (Linux/Darwin on amd64/arm64)
- Automated releases via GitHub Actions
- All dependencies updated to latest versions
- Fixed deprecated Prometheus APIs

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/Liquescent-Development/ovn_exporter/releases):

```bash
# Linux amd64
wget https://github.com/Liquescent-Development/ovn_exporter/releases/download/v2.6.0/ovn-exporter_2.6.0_linux_amd64.tar.gz
tar xvzf ovn-exporter_2.6.0_linux_amd64.tar.gz
sudo mv ovn-exporter /usr/local/bin/

# Linux arm64
wget https://github.com/Liquescent-Development/ovn_exporter/releases/download/v2.6.0/ovn-exporter_2.6.0_linux_arm64.tar.gz
tar xvzf ovn-exporter_2.6.0_linux_arm64.tar.gz
sudo mv ovn-exporter /usr/local/bin/
```

### Build from Source

```bash
git clone https://github.com/Liquescent-Development/ovn_exporter.git
cd ovn_exporter
make BUILD_OS=linux BUILD_ARCH=amd64
sudo make deploy BUILD_OS=linux BUILD_ARCH=amd64
```

### Systemd Service

Create a systemd service file `/etc/systemd/system/ovn-exporter.service`:

```ini
[Unit]
Description=OVN Exporter
After=network.target

[Service]
Type=simple
User=ovn-exporter
Group=openvswitch
ExecStart=/usr/local/bin/ovn-exporter
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Then enable and start the service:

```bash
sudo useradd -r -s /bin/false ovn-exporter
sudo usermod -a -G openvswitch ovn-exporter
sudo systemctl daemon-reload
sudo systemctl enable ovn-exporter
sudo systemctl start ovn-exporter
```

## Configuration

The exporter can be configured via command-line flags. Run `ovn-exporter --help` to see all available options.

### Common Flags

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--web.listen-address` | `:9476` | Addresses on which to expose metrics and web interface. Repeatable for multiple addresses. Examples: `:9100` or `[::1]:9100` for http, `vsock://:9100` for vsock |
| `--web.telemetry-path` | `/metrics` | Path under which to expose metrics |
| `--web.config.file` | "" | Path to configuration file that can enable TLS or authentication. See [prometheus documentation](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md) |
| `--ovn.timeout` | `2` | Timeout on gRPC requests to OVN (seconds) |
| `--ovn.poll-interval` | `15` | Minimum interval between collections from OVN server (seconds) |
| `--log.level` | `info` | Logging severity level (debug, info, warn, error) |
| `--log.format` | `logfmt` | Output format of log messages. One of: [logfmt, json] |
| `--version` | `false` | Show version information and exit |

### OVS Database Configuration

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--database.vswitch.name` | `Open_vSwitch` | Name of OVS database |
| `--database.vswitch.socket.remote` | `unix:/var/run/openvswitch/db.sock` | JSON-RPC unix socket to OVS db |
| `--database.vswitch.file.data.path` | `/etc/openvswitch/conf.db` | OVS database file |
| `--database.vswitch.file.log.path` | `/var/log/openvswitch/ovsdb-server.log` | OVS database log file |
| `--database.vswitch.file.pid.path` | `/var/run/openvswitch/ovsdb-server.pid` | OVS database process ID file |
| `--database.vswitch.file.system.id.path` | `/etc/openvswitch/system-id.conf` | OVS system ID file |

### OVN Northbound Database Configuration

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--database.northbound.name` | `OVN_Northbound` | Name of OVN Northbound database |
| `--database.northbound.socket.remote` | `unix:/run/openvswitch/ovnnb_db.sock` | JSON-RPC unix socket to OVN NB db |
| `--database.northbound.socket.control` | `unix:/run/openvswitch/ovnnb_db.ctl` | JSON-RPC unix socket to OVN NB app |
| `--database.northbound.file.data.path` | `/var/lib/openvswitch/ovnnb_db.db` | OVN NB database file |
| `--database.northbound.file.log.path` | `/var/log/openvswitch/ovsdb-server-nb.log` | OVN NB database log file |
| `--database.northbound.file.pid.path` | `/run/openvswitch/ovnnb_db.pid` | OVN NB database process ID file |
| `--database.northbound.port.default` | `6641` | OVN NB database network socket port |
| `--database.northbound.port.ssl` | `6631` | OVN NB database network socket secure port |
| `--database.northbound.port.raft` | `6643` | OVN NB database network port for clustering (raft) |

### OVN Southbound Database Configuration

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--database.southbound.name` | `OVN_Southbound` | Name of OVN Southbound database |
| `--database.southbound.socket.remote` | `unix:/run/openvswitch/ovnsb_db.sock` | JSON-RPC unix socket to OVN SB db |
| `--database.southbound.socket.control` | `unix:/run/openvswitch/ovnsb_db.ctl` | JSON-RPC unix socket to OVN SB app |
| `--database.southbound.file.data.path` | `/var/lib/openvswitch/ovnsb_db.db` | OVN SB database file |
| `--database.southbound.file.log.path` | `/var/log/openvswitch/ovsdb-server-sb.log` | OVN SB database log file |
| `--database.southbound.file.pid.path` | `/run/openvswitch/ovnsb_db.pid` | OVN SB database process ID file |
| `--database.southbound.port.default` | `6642` | OVN SB database network socket port |
| `--database.southbound.port.ssl` | `6632` | OVN SB database network socket secure port |
| `--database.southbound.port.raft` | `6644` | OVN SB database network port for clustering (raft) |

### Service Configuration

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--service.ovn.northd.file.log.path` | `/var/log/openvswitch/ovn-northd.log` | OVN northd daemon log file |
| `--service.ovn.northd.file.pid.path` | `/run/openvswitch/ovn-northd.pid` | OVN northd daemon process ID file |
| `--service.ovn.northd.socket.control` | `unix:/run/openvswitch/ovn-northd.ctl` | JSON-RPC unix socket to OVN northd app |

### Example Configuration

```bash
# Basic configuration
ovn-exporter \
  --web.listen-address=:9476 \
  --ovn.poll-interval=30 \
  --log.level=info

# Custom paths configuration
ovn-exporter \
  --web.listen-address=:9476 \
  --system.run.dir=/custom/run/openvswitch \
  --database.northbound.socket.remote=unix:/custom/run/openvswitch/ovnnb_db.sock \
  --database.southbound.socket.remote=unix:/custom/run/openvswitch/ovnsb_db.sock \
  --log.level=debug
```

## Metrics

The exporter provides comprehensive metrics about your OVN infrastructure. See [METRICS.md](METRICS.md) for a complete list of available metrics.

Example metrics:
- `ovn_up` - Whether OVN stack is up (1) or down (0)
- `ovn_info` - OVN stack information
- `ovn_cluster_*` - Cluster health and Raft consensus metrics
- `ovn_logical_switch_*` - Logical network topology metrics
- `ovn_chassis_info` - Chassis status and information

## Grafana Dashboards

Pre-built Grafana dashboards are available in the [`dashboards/`](dashboards/) directory to help you visualize and monitor your OVN infrastructure:

### 🏗️ OVN System Overview
**File**: [`ovn-system-overview.json`](dashboards/ovn-system-overview.json)

A comprehensive infrastructure monitoring dashboard for system operators featuring:
- Real-time OVN stack health status
- System availability and error rate monitoring
- Infrastructure topology overview (chassis, switches, routers, ports)
- Database and log file size trends
- Component-wise log event analysis
- Active chassis information table

**Target Audience**: System operators, infrastructure teams, NOC engineers

### 📊 OVN SLA/SLO Performance
**File**: [`ovn-sla-performance.json`](dashboards/ovn-sla-performance.json)

A customer experience focused dashboard for tracking service level objectives:
- SLA uptime compliance gauges with configurable time periods
- Network availability percentage with threshold alerting
- Error rate monitoring and SLA impact analysis
- Response time tracking and performance metrics
- Active resource monitoring (chassis, tunnels)
- SLA compliance trends and violation tracking
- Memory usage analysis by component

**Target Audience**: Service delivery teams, customer success managers, SLA compliance officers

**SLA Thresholds**:
- 🟢 **Excellent**: ≥99.9% uptime
- 🟡 **Warning**: 99.0-99.9% uptime
- 🔴 **Critical**: <99.0% uptime

### Quick Import

1. Open Grafana and navigate to **Dashboards** > **Import**
2. Upload the JSON file or paste the dashboard content
3. Select your Prometheus datasource
4. Configure any variables (datasource, SLA period)
5. Save and enjoy!

For detailed setup instructions, customization options, and alerting integration, see [dashboards/README.md](dashboards/README.md).

## Prometheus Configuration

Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'ovn'
    static_configs:
      - targets: ['localhost:9476']
```

## Development

### Requirements
- Go 1.24+
- Access to OVN/OVS Unix sockets (typically requires root or openvswitch group membership)

### Building
```bash
# Build for current platform
make BUILD_OS=linux BUILD_ARCH=amd64

# Run tests
make test BUILD_OS=linux BUILD_ARCH=amd64

# Create test OVN environment
make ovn

# Clean up test environment
make del-ovn
```

### Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Security

This version includes several security improvements:
- HTTP server configured with proper timeouts
- Security headers (X-Content-Type-Options, X-Frame-Options, CSP)
- Removed debug/profiling endpoints
- All dependencies regularly updated

If you discover a security vulnerability, please email security@liquescent.dev

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

This is a fork of the original [ovn_exporter](https://github.com/greenpau/ovn_exporter) by Paul Greenberg (@greenpau). Thank you for creating and maintaining the original project!

## Support

For issues and questions:
- Open an issue on [GitHub](https://github.com/Liquescent-Development/ovn_exporter/issues)
- Check the [METRICS.md](METRICS.md) for metric documentation