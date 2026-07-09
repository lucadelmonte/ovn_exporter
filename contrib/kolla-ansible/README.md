# OVN Exporter for Kolla-Ansible Deployments

This directory contains configuration files to run ovn-exporter with OpenStack deployments managed by Kolla-Ansible.

## Two ways to deploy

- **Automated (recommended):** the [`ovs-ovn-exporter` Ansible role](roles/ovs-ovn-exporter/)
  installs and configures **both** ovn-exporter and its companion
  [ovs-exporter](https://github.com/lucadelmonte/ovs_exporter) across your fleet
  (systemd unit, environment file, Kolla container dependencies, versioned
  binary download). Start here if you use Ansible. An example playbook lives at
  [`ovs-ovn-exporter.yml`](ovs-ovn-exporter.yml).
- **Manual:** the step-by-step instructions below, using the standalone
  `ovn-exporter.env` / `ovn-exporter-kolla.conf` / `ovn-exporter.yml` files in
  this directory. Use this if you are not running Ansible or just want to
  understand what the role does.

## Problem

Kolla-Ansible deploys OVN components (NB database, SB database, northd) in Docker containers. The default ovn-exporter configuration expects these components to be running on the host with standard paths. This guide explains how to configure both Kolla and ovn-exporter to work together.

## Changes Required

### 1. Configure Kolla-Ansible to Expose /run/ovn

OVN containers keep their socket, control, and PID files in `/run/ovn/` inside the container. To allow the exporter (running on the host) to access these files, configure Kolla to mount this directory.

Copy `ovn-exporter.yml` to your Kolla configuration:

```bash
cp ovn-exporter.yml /etc/kolla/globals.d/ovn-exporter.yml
```

Or add to your existing `group_vars/all.yml`:

```yaml
ovn_nb_db_extra_volumes:
  - "/run/ovn:/run/ovn:rw"

ovn_sb_db_extra_volumes:
  - "/run/ovn:/run/ovn:rw"

ovn_northd_extra_volumes:
  - "/run/ovn:/run/ovn:rw"
```

Then reconfigure the OVN containers:

```bash
kolla-ansible -i inventory reconfigure -t ovn
```

### 2. Install the Exporter

Download and extract the exporter binary:

```bash
VERSION=$(curl -s https://api.github.com/repos/lucadelmonte/ovn_exporter/releases/latest | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
echo "Latest version: $VERSION"
wget "https://github.com/lucadelmonte/ovn_exporter/releases/download/v$VERSION/ovn-exporter_${VERSION}_linux_amd64.tar.gz"
mkdir ovn-exporter
tar -xzf ovn-exporter_${VERSION}_linux_amd64.tar.gz -C ovn-exporter
cd ovn-exporter
```

### 3. Configure for Kolla-Ansible

Download and install the environment file with Kolla-specific paths:

```bash
# For RHEL/CentOS
sudo wget -O /etc/sysconfig/ovn-exporter https://raw.githubusercontent.com/lucadelmonte/ovn_exporter/v$VERSION/contrib/kolla-ansible/ovn-exporter.env

# For Debian/Ubuntu
sudo wget -O /etc/default/ovn-exporter https://raw.githubusercontent.com/lucadelmonte/ovn_exporter/v$VERSION/contrib/kolla-ansible/ovn-exporter.env
```

Download and install systemd drop-in override for Kolla container dependencies:

```bash
sudo mkdir -p /etc/systemd/system/ovn-exporter.service.d/
sudo wget -O /etc/systemd/system/ovn-exporter.service.d/ovn-exporter-kolla.conf https://raw.githubusercontent.com/lucadelmonte/ovn_exporter/v$VERSION/contrib/kolla-ansible/ovn-exporter-kolla.conf
```

### 4. Start the Service

Run the installation script to install the binary and default systemd service:

```bash
sudo ./install.sh
```


### 5. Verify

```bash
# Check service status
systemctl status ovn-exporter

# Check metrics
curl -s http://localhost:9476/metrics | head -20

# Count available metrics
curl -s http://localhost:9476/metrics | wc -l
```

## Path Mapping Reference

| Component | Default Path | Kolla Path |
|-----------|--------------|------------|
| NB socket | `unix:/run/openvswitch/ovnnb_db.sock` | `unix:/run/ovn/ovnnb_db.sock` |
| NB control | `unix:/run/openvswitch/ovnnb_db.ctl` | `unix:/run/ovn/ovnnb_db.ctl` |
| NB PID | `/run/openvswitch/ovnnb_db.pid` | `/run/ovn/ovnnb_db.pid` |
| NB data | `/var/lib/openvswitch/ovnnb_db.db` | `/var/lib/docker/volumes/ovn_nb_db/_data/ovnnb.db` |
| NB log | `/var/log/openvswitch/ovsdb-server-nb.log` | `/var/log/kolla/openvswitch/ovn-nb-db.log` |
| SB socket | `unix:/run/openvswitch/ovnsb_db.sock` | `unix:/run/ovn/ovnsb_db.sock` |
| SB control | `unix:/run/openvswitch/ovnsb_db.ctl` | `unix:/run/ovn/ovnsb_db.ctl` |
| SB PID | `/run/openvswitch/ovnsb_db.pid` | `/run/ovn/ovnsb_db.pid` |
| SB data | `/var/lib/openvswitch/ovnsb_db.db` | `/var/lib/docker/volumes/ovn_sb_db/_data/ovnsb.db` |
| SB log | `/var/log/openvswitch/ovsdb-server-sb.log` | `/var/log/kolla/openvswitch/ovn-sb-db.log` |
| northd PID | `/run/openvswitch/ovn-northd.pid` | `/run/ovn/ovn-northd.pid` |
| northd log | `/var/log/openvswitch/ovn-northd.log` | `/var/log/kolla/openvswitch/ovn-northd.log` |

## Deployment Notes

### Compute Nodes vs Network Nodes

- **ovn-exporter** → Deploy on **network/controller nodes** (OVN central)
  - Monitors OVN northbound/southbound databases and northd
  - Port: 9476

- **ovs-exporter** → Deploy on **compute nodes** and **network/controller nodes**
  - Monitors local OVS bridges, flows, and ovn-controller
  - Port: 9475

Both exporters complement each other for complete OVN/OVS visibility.

## Notes

- The systemd unit depends on the OVN NB/SB DB containers (required) and the northd container (wanted); OVS is not required
- `system_id` and `hostname` labels are derived from `os.Hostname()`; `system_type`/`system_version` come from `/etc/os-release`; NB/SB schema versions come from the OVN databases
- Metrics are exposed on port 9476 by default
