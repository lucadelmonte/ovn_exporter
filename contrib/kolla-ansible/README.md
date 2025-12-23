# OVN Exporter for Kolla-Ansible Deployments

This directory contains configuration files to run ovn-exporter with OpenStack deployments managed by Kolla-Ansible.

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
wget https://github.com/lucadelmonte/ovn_exporter/releases/download/v2.3.0/ovn-exporter_2.3.0_linux_amd64.tar.gz
tar -xzf ovn-exporter_2.3.0_linux_amd64.tar.gz
cd ovn-exporter_2.3.0_linux_amd64
```

Run the installation script to install the binary and default systemd service:

```bash
sudo ./install.sh
```

### 3. Configure for Kolla-Ansible

Download and install the environment file with Kolla-specific paths:

```bash
# For RHEL/CentOS
sudo wget -O /etc/sysconfig/ovn-exporter https://raw.githubusercontent.com/lucadelmonte/ovn_exporter/v2.3.0/contrib/kolla-ansible/ovn-exporter.env

# For Debian/Ubuntu
sudo wget -O /etc/default/ovn-exporter https://raw.githubusercontent.com/lucadelmonte/ovn_exporter/v2.3.0/contrib/kolla-ansible/ovn-exporter.env
```

Download and install systemd drop-in override for Kolla container dependencies:

```bash
sudo mkdir -p /etc/systemd/system/ovn-exporter.service.d/
sudo wget -O /etc/systemd/system/ovn-exporter.service.d/ovn-exporter-kolla.conf https://raw.githubusercontent.com/lucadelmonte/ovn_exporter/v2.3.0/contrib/kolla-ansible/ovn-exporter-kolla.conf
```

### 4. Start the Service

```bash
sudo systemctl daemon-reload
sudo systemctl enable ovn-exporter
sudo systemctl start ovn-exporter
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
| OVS data | `/etc/openvswitch/conf.db` | `/var/lib/docker/volumes/openvswitch_db/_data/conf.db` |
| OVS log | `/var/log/openvswitch/ovsdb-server.log` | `/var/log/kolla/openvswitch/ovsdb-server.log` |
| vswitchd log | `/var/log/openvswitch/ovs-vswitchd.log` | `/var/log/kolla/openvswitch/ovs-vswitchd.log` |

## Notes

- The systemd unit depends on `kolla-openvswitch_db-container.service` (required) and OVN containers (wanted)
- System information is queried directly from the OVS database - no manual system-id file needed
- Metrics are exposed on port 9476 by default
