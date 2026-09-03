// Copyright 2018 Paul Greenberg (greenpau@outlook.com)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ovn_exporter

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/greenpau/ovsdb"
	"github.com/greenpau/versioned"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
)

const (
	namespace = "ovn"
)

var (
	app        *versioned.PackageManager
	appVersion string
	gitBranch  string
	gitCommit  string
	buildUser  string
	buildDate  string
)

func init() {
	app = versioned.NewPackageManager("ovn-exporter")
	app.Description = "Prometheus Exporter for Open Virtual Network (OVN)"
	app.Documentation = "https://github.com/Liquescent-Development/ovn_exporter/"
	app.SetVersion(appVersion, "2.6.0")
	app.SetGitBranch(gitBranch, "")
	app.SetGitCommit(gitCommit, "")
	app.SetBuildUser(buildUser, "")
	app.SetBuildDate(buildDate, "")
}

var (
	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Is OVN stack up (1) or is it down (0).",
		nil, nil,
	)
	info = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "info"),
		"This metric provides basic information about OVN stack. It is always set to 1.",
		[]string{
			"system_id",
			"hostname",
			"system_type",
			"system_version",
			"nb_schema_version",
			"sb_schema_version",
		}, nil,
	)
	requestErrors = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "failed_requests_total"),
		"The total number of failed requests to OVN stack.",
		[]string{"system_id"}, nil,
	)
	requestsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "requests_total"),
		"The total number of requests to OVN stack.",
		[]string{"system_id"}, nil,
	)
	requestsSuccessful = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "successful_requests_total"),
		"The total number of successful requests to OVN stack.",
		[]string{"system_id"}, nil,
	)
	nextPoll = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "next_poll_timestamp_seconds"),
		"The timestamp of the next potential poll of OVN stack in seconds.",
		[]string{"system_id"}, nil,
	)
	logFileSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "log_file_size_bytes"),
		"The size of a log file associated with an OVN component in bytes.",
		[]string{"system_id", "component", "filename"}, nil,
	)
	logEventStat = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "log_events_total"),
		"The total number of recorded log messages associated with an OVN component by log severity level and source.",
		[]string{"system_id", "component", "severity", "source"}, nil,
	)
	dbFileSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "db_file_size_bytes"),
		"The size of a database file associated with an OVN component in bytes.",
		[]string{"system_id", "component", "filename"}, nil,
	)
	chassisInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "chassis_nb_cfg_timestamp_milliseconds"),
		"The nb_cfg_timestamp from OVN Chassis_Private table in milliseconds. Value is 0 if chassis has no entry in Chassis_Private (not reporting).",
		[]string{"system_id", "uuid", "name", "ip"}, nil,
	)
	chassisNbCfg = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "chassis_nb_cfg"),
		"The nb_cfg (configuration sequence number) from OVN Chassis_Private table. Value is 0 if chassis has no entry in Chassis_Private (not reporting).",
		[]string{"system_id", "uuid", "name", "ip"}, nil,
	)
	logicalSwitchInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_switch_info"),
		"The information about OVN logical switch. This metric is always up (1).",
		[]string{"system_id", "uuid", "name"}, nil,
	)
	logicalSwitchExternalIDs = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_switch_external_id"),
		"Provides the external IDs and values associated with OVN logical switches. This metric is always up (1).",
		[]string{"system_id", "uuid", "key", "value"}, nil,
	)
	logicalSwitchPortBinding = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_switch_port_binding"),
		"Provides the association between a logical switch and a logical switch port. This metric is always up (1).",
		[]string{"system_id", "uuid", "port"}, nil,
	)
	logicalSwitchTunnelKey = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_switch_tunnel_key"),
		"The value of the tunnel key associated with the logical switch.",
		[]string{"system_id", "uuid"}, nil,
	)
	logicalSwitchPorts = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_switch_ports"),
		"The number of logical switch ports connected to the OVN logical switch.",
		[]string{"system_id", "uuid"}, nil,
	)
	logicalSwitchPortInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_switch_port_info"),
		"The information about OVN logical switch port. This metric is always up (1).",
		[]string{
			"system_id",
			"uuid",
			"name",
			"chassis",
			"logical_switch",
			"datapath",
			"port_binding",
			"mac_address",
			"ip_address",
		}, nil,
	)
	logicalSwitchPortTunnelKey = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_switch_port_tunnel_key"),
		"The value of the tunnel key associated with the logical switch port.",
		[]string{"system_id", "uuid"}, nil,
	)
	portBindingCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "port_binding_count"),
		"Number of OVN Southbound port bindings scheduled on a chassis, broken down by port binding type. chassis is the chassis UUID and chassis_name its human-readable name. Unscheduled bindings are reported with chassis=\"unbound\"; plain VIF ports (empty OVN type) are reported with type=\"vif\".",
		[]string{"system_id", "chassis", "chassis_name", "type"}, nil,
	)
	gatewayPortInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "gateway_port_info"),
		"Information about chassis-pinned OVN gateway port bindings (type=chassisredirect or type=l3gateway). Always 1. chassis_name is the human-readable name of the chassis referenced by chassis. The Neutron router UUID is exposed via router_name when the binding carries a neutron:router_name external_id.",
		[]string{"system_id", "uuid", "chassis", "chassis_name", "type", "logical_port", "datapath", "router_name"}, nil,
	)
	logicalRouterInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_router_info"),
		"The information about OVN logical router. This metric is always up (1).",
		[]string{"system_id", "uuid", "name"}, nil,
	)
	logicalRouterExternalIDs = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_router_external_id"),
		"Provides the external IDs and values associated with OVN logical routers. This metric is always up (1).",
		[]string{"system_id", "uuid", "key", "value"}, nil,
	)
	logicalRouterPorts = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_router_ports"),
		"The number of logical router ports connected to the OVN logical router.",
		[]string{"system_id", "uuid"}, nil,
	)
	logicalRouterStaticRoutes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_router_static_routes"),
		"The number of static routes configured on the OVN logical router.",
		[]string{"system_id", "uuid"}, nil,
	)
	logicalRouterNatRules = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_router_nat_rules"),
		"The number of NAT rules configured on the OVN logical router.",
		[]string{"system_id", "uuid"}, nil,
	)
	logicalRouterLoadBalancers = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_router_load_balancers"),
		"The number of load balancers associated with the OVN logical router.",
		[]string{"system_id", "uuid"}, nil,
	)
	logicalRouterPolicies = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "logical_router_policies"),
		"The number of policies configured on the OVN logical router.",
		[]string{"system_id", "uuid"}, nil,
	)
	networkPortUp = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "network_port"),
		"The TCP port used for database connection. If the value is 0, then the port is not in use.",
		[]string{"system_id", "component", "usage"}, nil,
	)
	covAvg = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "coverage_avg"),
		"The average rate of the number of times particular events occur during a OVSDB daemon's runtime.",
		[]string{"system_id", "component", "event", "interval"}, nil,
	)
	covTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "coverage_total"),
		"The total number of times particular events occur during a OVSDB daemon's runtime.",
		[]string{"system_id", "component", "event"}, nil,
	)
	memUsage = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "memory_usage_bytes"),
		"The memory usage in bytes.",
		[]string{"system_id", "component", "facility"}, nil,
	)

	clusterEnabled = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_enabled"),
		"Is OVN clustering enabled (1) or not (0).",
		[]string{"system_id", "component"}, nil,
	)
	clusterPeerInConnInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_inbound_peer_connected"),
		"This metric appears when a cluster peer is connected to this server. This metric is always 1.",
		[]string{"system_id", "component", "server_id", "cluster_id", "peer_id", "peer_address"}, nil,
	)
	clusterPeerOutConnInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_outbound_peer_connected"),
		"This metric appears when this server connects to a cluster peer. This metric is always 1.",
		[]string{"system_id", "component", "server_id", "cluster_id", "peer_id", "peer_address"}, nil,
	)
	clusterPeerInConnTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_inbound_peer_conn_total"),
		"The total number of outbound connections to cluster peers.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterPeerOutConnTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_outbound_peer_conn_total"),
		"The total number of inbound connections from cluster peers.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterPeerCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_peer_count"),
		"The total number of peers in this server's cluster.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterRole = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_role"),
		"The role of this server in the cluster. The values are: 3 - leader, 2 - candidate, 1 - follower, 0 - other.",
		[]string{"system_id", "component", "server_id", "server_uuid", "cluster_id", "cluster_uuid"}, nil,
	)
	clusterStatus = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_status"),
		"The status of this server in the cluster. The values are: 1 - cluster member, 0 - other.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterTerm = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_term"),
		"The current raft term known by this server.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterNotCommittedEntryCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_uncommitted_entry_count"),
		"The number of raft entries not yet committed by this server.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterNotAppliedEntryCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_pending_entry_count"),
		"The number of raft entries not yet applied by this server.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterPeerNextIndex = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_peer_next_index"),
		"The raft's next index associated with this cluster peer.",
		[]string{"system_id", "component", "server_id", "cluster_id", "peer_id"}, nil,
	)
	clusterPeerMatchIndex = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_peer_match_index"),
		"The raft's match index associated with this cluster peer.",
		[]string{"system_id", "component", "server_id", "cluster_id", "peer_id"}, nil,
	)
	clusterLeaderSelf = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_leader_self"),
		"Is this server consider itself a leader (1) or not (0).",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterVoteSelf = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_vote_self"),
		"Is this server voted itself as a leader (1) or not (0).",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterNextIndex = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_next_index"),
		"The raft's next index associated with this server.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterMatchIndex = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_match_index"),
		"The raft's match index associated with this server.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterLogLowIndex = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_log_low_index"),
		"The raft's low number associated with this server.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterLogHighIndex = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_log_high_index"),
		"The raft's low number associated with this server.",
		[]string{"system_id", "component", "server_id", "cluster_id"}, nil,
	)
	clusterGroup = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_group"),
		"The cluster group in which this server participates. It is a combination of SB and NB cluster IDs. This metric is always up (1).",
		[]string{"system_id", "cluster_group"}, nil,
	)
)

// Exporter collects OVN data from the given server and exports them using
// the prometheus metrics package.
type Exporter struct {
	sync.RWMutex
	Client               *ovsdb.OvnClient
	timeout              int
	pollInterval         int64
	errors               int64
	totalRequests        int64
	successfulRequests   int64
	requestsLocker       sync.RWMutex
	nextCollectionTicker int64
	metrics              []prometheus.Metric
	logger               *slog.Logger
}

type Options struct {
	Timeout int
	Logger  *slog.Logger
}

// NewExporter returns an initialized Exporter.
func NewExporter(opts Options) (*Exporter, error) {
	version.Version = appVersion
	version.Revision = gitCommit
	version.Branch = gitBranch
	version.BuildUser = buildUser
	version.BuildDate = buildDate
	e := Exporter{
		timeout: opts.Timeout,
		logger:  opts.Logger,
	}
	client := ovsdb.NewOvnClient()
	client.Timeout = opts.Timeout
	e.Client = client
	e.Client.GetSystemID()
	return &e, nil
}

// ExporterPerformClientCalls creates client connection.
func ExporterPerformClientCalls(e *Exporter) (*Exporter, error) {
	e.logger.Debug(
		"msg", "NewExporter() calls Connect()",
		"system_id", e.Client.System.ID,
	)
	if err := e.Client.Connect(); err != nil {
		return e, err
	}
	e.logger.Debug(
		"msg", "NewExporter() calls GetSystemInfo()",
		"system_id", e.Client.System.ID,
	)
	if err := e.Client.GetSystemInfo(); err != nil {
		return e, err
	}
	e.logger.Debug(
		"msg", "NewExporter() initialized successfully",
		"system_id", e.Client.System.ID,
	)
	return e, nil
}

// Describe describes all the metrics ever exported by the OVN exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- info
	ch <- requestErrors
	ch <- requestsTotal
	ch <- requestsSuccessful
	ch <- nextPoll
	ch <- logFileSize
	ch <- dbFileSize
	ch <- logEventStat
	ch <- chassisInfo
	ch <- chassisNbCfg
	ch <- logicalSwitchInfo
	ch <- logicalSwitchExternalIDs
	ch <- logicalSwitchPorts
	ch <- logicalSwitchPortBinding
	ch <- logicalSwitchTunnelKey
	ch <- logicalSwitchPortInfo
	ch <- logicalSwitchPortTunnelKey
	ch <- portBindingCount
	ch <- gatewayPortInfo
	ch <- logicalRouterInfo
	ch <- logicalRouterExternalIDs
	ch <- logicalRouterPorts
	ch <- logicalRouterStaticRoutes
	ch <- logicalRouterNatRules
	ch <- logicalRouterLoadBalancers
	ch <- logicalRouterPolicies
	ch <- networkPortUp
	ch <- covAvg
	ch <- covTotal
	ch <- memUsage
	ch <- clusterEnabled
	ch <- clusterRole
	ch <- clusterStatus
	ch <- clusterTerm
	ch <- clusterNotCommittedEntryCount
	ch <- clusterNotAppliedEntryCount
	ch <- clusterNextIndex
	ch <- clusterMatchIndex
	ch <- clusterLogLowIndex
	ch <- clusterLogHighIndex
	ch <- clusterLeaderSelf
	ch <- clusterVoteSelf
	ch <- clusterPeerCount
	ch <- clusterPeerInConnTotal
	ch <- clusterPeerOutConnTotal
	ch <- clusterPeerNextIndex
	ch <- clusterPeerMatchIndex
	ch <- clusterPeerInConnInfo
	ch <- clusterPeerOutConnInfo
	ch <- clusterGroup
}

// IncrementErrorCounter increases the counter of failed queries
// to OVN server.
func (e *Exporter) IncrementErrorCounter() {
	e.requestsLocker.Lock()
	defer e.requestsLocker.Unlock()
	atomic.AddInt64(&e.errors, 1)
	atomic.AddInt64(&e.totalRequests, 1)
}

// IncrementRequestCounter increases the counter of total requests
// to OVN server.
func (e *Exporter) IncrementRequestCounter() {
	e.requestsLocker.Lock()
	defer e.requestsLocker.Unlock()
	atomic.AddInt64(&e.totalRequests, 1)
}

// IncrementSuccessCounter increases the counter of successful queries
// to OVN server.
func (e *Exporter) IncrementSuccessCounter() {
	e.requestsLocker.Lock()
	defer e.requestsLocker.Unlock()
	atomic.AddInt64(&e.successfulRequests, 1)
	atomic.AddInt64(&e.totalRequests, 1)
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.GatherMetrics()
	e.logger.Debug(
		"msg", "Collect() calls RLock()",
		"system_id", e.Client.System.ID,
	)
	e.RLock()
	defer e.RUnlock()
	if len(e.metrics) == 0 {
		e.logger.Debug(
			"msg", "Collect() no metrics found",
			"system_id", e.Client.System.ID,
		)
		ch <- prometheus.MustNewConstMetric(
			up,
			prometheus.GaugeValue,
			0,
		)
		ch <- prometheus.MustNewConstMetric(
			info,
			prometheus.GaugeValue,
			1,
			e.Client.System.ID, e.Client.System.Hostname,
			e.Client.System.Type, e.Client.System.Version,
			e.Client.Database.Northbound.Schema.Version, e.Client.Database.Southbound.Schema.Version,
		)
		ch <- prometheus.MustNewConstMetric(
			requestErrors,
			prometheus.CounterValue,
			float64(e.errors),
			e.Client.System.ID,
		)
		ch <- prometheus.MustNewConstMetric(
			requestsTotal,
			prometheus.CounterValue,
			float64(e.totalRequests),
			e.Client.System.ID,
		)
		ch <- prometheus.MustNewConstMetric(
			requestsSuccessful,
			prometheus.CounterValue,
			float64(e.successfulRequests),
			e.Client.System.ID,
		)
		ch <- prometheus.MustNewConstMetric(
			nextPoll,
			prometheus.CounterValue,
			float64(e.nextCollectionTicker),
			e.Client.System.ID,
		)
		return
	}
	e.logger.Debug(
		"msg", "Collect() sends metrics to a shared channel",
		"system_id", e.Client.System.ID,
		"metric_count", len(e.metrics),
	)
	for _, m := range e.metrics {
		ch <- m
	}
}

// GatherMetrics collect data from OVN server and stores them
// as Prometheus metrics.
func (e *Exporter) GatherMetrics() {
	e.logger.Debug(
		"msg", "GatherMetrics() called",
		"system_id", e.Client.System.ID,
	)
	if time.Now().Unix() < e.nextCollectionTicker {
		return
	}
	e.Lock()
	e.logger.Debug(
		"msg", "GatherMetrics() locked",
		"system_id", e.Client.System.ID,
	)
	defer e.Unlock()
	if len(e.metrics) > 0 {
		e.metrics = e.metrics[:0]
		e.logger.Debug(
			"msg", "GatherMetrics() cleared metrics",
			"system_id", e.Client.System.ID,
		)
	}
	upValue := 1
	isClusterEnabled := false

	var err error

	err = e.Client.GetSystemInfo()
	if err != nil {
		e.logger.Error(
			"msg", "GetSystemInfo() failed",
			"system_id", e.Client.System.ID,
			"error", err.Error(),
		)
		e.IncrementErrorCounter()
		upValue = 0
	} else {
		e.IncrementSuccessCounter()
		e.logger.Debug(
			"msg", "GetSystemInfo() successful",
			"system_id", e.Client.System.ID,
		)
	}

	components := []string{
		"ovsdb-server-southbound",
		"ovsdb-server-northbound",
		"ovn-northd",
	}
	for _, component := range components {
		e.logger.Debug(
			"msg", "GatherMetrics() calls GetProcessInfo()",
			"component", component,
			"system_id", e.Client.System.ID,
		)
		if _, err := e.Client.GetProcessInfo(component); err != nil {
			e.logger.Error(
				"msg", "GetProcessInfo() failed",
				"component", component,
				"system_id", e.Client.System.ID,
				"error", err.Error(),
			)
			e.IncrementErrorCounter()
			upValue = 0
		} else {
			e.IncrementSuccessCounter()
		}
		e.logger.Debug(
			"msg", "GatherMetrics() completed GetProcessInfo()",
			"component", component,
			"system_id", e.Client.System.ID,
		)
	}

	components = []string{
		"ovsdb-server-southbound",
		"ovsdb-server-northbound",
		"ovn-northd",
	}
	for _, component := range components {
		e.logger.Debug(
			"msg", "GatherMetrics() calls GetLogFileInfo()",
			"component", component,
			"system_id", e.Client.System.ID,
		)
		file, err := e.Client.GetLogFileInfo(component)
		if err != nil {
			e.logger.Error(
				"msg", "GetLogFileInfo() failed",
				"component", component,
				"system_id", e.Client.System.ID,
				"error", err.Error(),
			)
			e.IncrementErrorCounter()
			continue
		} else {
			e.IncrementSuccessCounter()
		}
		e.logger.Debug(
			"msg", "GatherMetrics() completed GetLogFileInfo()",
			"component", component,
			"system_id", e.Client.System.ID,
		)
		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			logFileSize,
			prometheus.GaugeValue,
			float64(file.Info.Size()),
			e.Client.System.ID,
			file.Component,
			file.Path,
		))
		e.logger.Debug(
			"msg", "GatherMetrics() calls GetLogFileEventStats()",
			"component", component,
			"system_id", e.Client.System.ID,
		)
		eventStats, err := e.Client.GetLogFileEventStats(component)
		if err != nil {
			e.logger.Error(
				"msg", "GetLogFileEventStats() failed",
				"component", component,
				"system_id", e.Client.System.ID,
				"error", err.Error(),
			)
			e.IncrementErrorCounter()
			continue
		} else {
			e.IncrementSuccessCounter()
		}
		e.logger.Debug(
			"msg", "GatherMetrics() completed GetLogFileEventStats()",
			"component", component,
			"system_id", e.Client.System.ID,
		)
		for sev, sources := range eventStats {
			for source, count := range sources {
				e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
					logEventStat,
					prometheus.GaugeValue,
					float64(count),
					e.Client.System.ID,
					component,
					sev,
					source,
				))
			}
		}
	}

	e.logger.Debug(
		"msg", "GatherMetrics() calls GetChassis()",
		"system_id", e.Client.System.ID,
	)
	chassisNameByUUID := map[string]string{}
	if vteps, err := e.Client.GetChassis(); err != nil {
		e.logger.Error(
			"msg", "GetChassis() failed",
			"southbound_db_name", e.Client.Database.Southbound.Name,
			"system_id", e.Client.System.ID,
			"error", err.Error(),
		)
		e.IncrementErrorCounter()
		upValue = 0
	} else {
		e.IncrementSuccessCounter()
		for _, vtep := range vteps {
			chassisNameByUUID[vtep.UUID] = vtep.Name
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				chassisInfo,
				prometheus.GaugeValue,
				float64(vtep.NbCfgTimestamp),
				e.Client.System.ID,
				vtep.UUID,
				vtep.Name,
				vtep.IPAddress.String(),
			))
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				chassisNbCfg,
				prometheus.GaugeValue,
				float64(vtep.NbCfg),
				e.Client.System.ID,
				vtep.UUID,
				vtep.Name,
				vtep.IPAddress.String(),
			))
		}
	}
	e.logger.Debug(
		"msg", "GatherMetrics() completed GetChassis()",
		"system_id", e.Client.System.ID,
	)

	e.logger.Debug(
		"msg", "GatherMetrics() calls GetLogicalSwitches()",
		"system_id", e.Client.System.ID,
	)
	lsws, err := e.Client.GetLogicalSwitches()
	if err != nil {
		e.logger.Error(
			"msg", "GetLogicalSwitches() failed",
			"southbound_db_name", e.Client.Database.Southbound.Name,
			"system_id", e.Client.System.ID,
			"error", err.Error(),
		)
		e.IncrementErrorCounter()
		upValue = 0
	} else {
		e.IncrementSuccessCounter()
		for _, lsw := range lsws {
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalSwitchInfo,
				prometheus.GaugeValue,
				1,
				e.Client.System.ID,
				lsw.UUID,
				lsw.Name,
			))
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalSwitchPorts,
				prometheus.GaugeValue,
				float64(len(lsw.Ports)),
				e.Client.System.ID,
				lsw.UUID,
			))
			if len(lsw.Ports) > 0 {
				for _, p := range lsw.Ports {
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						logicalSwitchPortBinding,
						prometheus.GaugeValue,
						1,
						e.Client.System.ID,
						lsw.UUID,
						p,
					))
				}
			}
			if len(lsw.ExternalIDs) > 0 {
				for k, v := range lsw.ExternalIDs {
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						logicalSwitchExternalIDs,
						prometheus.GaugeValue,
						1,
						e.Client.System.ID,
						lsw.UUID,
						k,
						v,
					))
				}
			}
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalSwitchTunnelKey,
				prometheus.GaugeValue,
				float64(lsw.TunnelKey),
				e.Client.System.ID,
				lsw.UUID,
			))
		}
	}
	e.logger.Debug(
		"msg", "GatherMetrics() completed GetLogicalSwitches()",
		"system_id", e.Client.System.ID,
	)

	e.logger.Debug(
		"msg", "GatherMetrics() calls GetLogicalSwitchPorts()",
		"system_id", e.Client.System.ID,
	)
	lswps, err := e.Client.GetLogicalSwitchPorts()
	if err != nil {
		e.logger.Error(
			"msg", "GetLogicalSwitchPorts() failed",
			"southbound_db_name", e.Client.Database.Southbound.Name,
			"system_id", e.Client.System.ID,
			"error", err.Error(),
		)
		e.IncrementErrorCounter()
		upValue = 0
	} else {
		e.IncrementSuccessCounter()
		for _, port := range lswps {
			macAddr := "<nil>"
			ipAddr := "<nil>"

			// Find first MAC address
			for _, a := range port.Addresses {
				if a.MacAddress != nil {
					macAddr = a.MacAddress.String()
					break
				}
			}

			// Find first IP address
			for _, a := range port.Addresses {
				if len(a.IPAddresses) > 0 {
					ipAddr = a.IPAddresses[0].String()
					break
				}
			}

			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalSwitchPortInfo,
				prometheus.GaugeValue,
				float64(1),
				e.Client.System.ID,
				port.UUID,
				port.Name,
				port.ChassisUUID,
				port.LogicalSwitchName,
				port.DatapathUUID,
				port.PortBindingUUID,
				macAddr,
				ipAddr,
			))
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalSwitchPortTunnelKey,
				prometheus.GaugeValue,
				float64(port.TunnelKey),
				e.Client.System.ID,
				port.UUID,
			))
		}
	}
	e.logger.Debug(
		"msg", "GatherMetrics() completed GetLogicalSwitchPorts()",
		"system_id", e.Client.System.ID,
	)

	// Gather Port_Binding metrics (chassis-level distribution of all binding
	// types, plus drill-down info for chassis-pinned gateway ports).
	e.logger.Debug(
		"msg", "GatherMetrics() calls GetPortBindings()",
		"system_id", e.Client.System.ID,
	)
	pbs, err := e.Client.GetPortBindings()
	if err != nil {
		e.logger.Error(
			"msg", "GetPortBindings() failed",
			"southbound_db_name", e.Client.Database.Southbound.Name,
			"system_id", e.Client.System.ID,
			"error", err.Error(),
		)
		e.IncrementErrorCounter()
		upValue = 0
	} else {
		e.IncrementSuccessCounter()
		pbCounts := make(map[[3]string]int)
		for _, pb := range pbs {
			chassisLabel := pb.ChassisUUID
			chassisName := chassisNameByUUID[pb.ChassisUUID]
			if chassisLabel == "" {
				chassisLabel = "unbound"
			}
			typeLabel := pb.Type
			if typeLabel == "" {
				typeLabel = "vif"
			}
			pbCounts[[3]string{chassisLabel, chassisName, typeLabel}]++

			if pb.Type == "chassisredirect" || pb.Type == "l3gateway" {
				e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
					gatewayPortInfo,
					prometheus.GaugeValue,
					float64(1),
					e.Client.System.ID,
					pb.UUID,
					pb.ChassisUUID,
					chassisName,
					pb.Type,
					pb.LogicalPort,
					pb.DatapathUUID,
					pb.ExternalIDs["neutron:router_name"],
				))
			}
		}
		for key, count := range pbCounts {
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				portBindingCount,
				prometheus.GaugeValue,
				float64(count),
				e.Client.System.ID,
				key[0],
				key[1],
				key[2],
			))
		}
	}
	e.logger.Debug(
		"msg", "GatherMetrics() completed GetPortBindings()",
		"system_id", e.Client.System.ID,
	)

	// Gather Logical Router metrics
	e.logger.Debug(
		"msg", "GatherMetrics() calls GetLogicalRouters()",
		"system_id", e.Client.System.ID,
	)
	routers, err := e.GetLogicalRouters()
	if err != nil {
		e.logger.Error(
			"msg", "GetLogicalRouters() failed",
			"northbound_db_name", e.Client.Database.Northbound.Name,
			"system_id", e.Client.System.ID,
			"error", err.Error(),
		)
		e.IncrementErrorCounter()
		upValue = 0
	} else {
		e.IncrementSuccessCounter()
		for _, router := range routers {
			// Basic router info metric
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalRouterInfo,
				prometheus.GaugeValue,
				1,
				e.Client.System.ID,
				router.UUID,
				router.Name,
			))

			// Router ports count
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalRouterPorts,
				prometheus.GaugeValue,
				float64(len(router.Ports)),
				e.Client.System.ID,
				router.UUID,
			))

			// Static routes count
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalRouterStaticRoutes,
				prometheus.GaugeValue,
				float64(len(router.StaticRoutes)),
				e.Client.System.ID,
				router.UUID,
			))

			// NAT rules count
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalRouterNatRules,
				prometheus.GaugeValue,
				float64(len(router.NAT)),
				e.Client.System.ID,
				router.UUID,
			))

			// Load balancers count
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalRouterLoadBalancers,
				prometheus.GaugeValue,
				float64(len(router.LoadBalancer)),
				e.Client.System.ID,
				router.UUID,
			))

			// Policies count
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				logicalRouterPolicies,
				prometheus.GaugeValue,
				float64(len(router.Policies)),
				e.Client.System.ID,
				router.UUID,
			))

			// External IDs
			if len(router.ExternalIDs) > 0 {
				for k, v := range router.ExternalIDs {
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						logicalRouterExternalIDs,
						prometheus.GaugeValue,
						1,
						e.Client.System.ID,
						router.UUID,
						k,
						v,
					))
				}
			}
		}
	}
	e.logger.Debug(
		"msg", "GatherMetrics() completed GetLogicalRouters()",
		"system_id", e.Client.System.ID,
	)

	northClusterID := ""
	southClusterID := ""

	components = []string{
		"ovsdb-server-southbound",
		"ovsdb-server-northbound",
	}

	for _, component := range components {
		e.logger.Debug(
			"msg", "GatherMetrics() calls AppListCommands()",
			"component", component,
			"system_id", e.Client.System.ID,
		)
		if cmds, err := e.Client.AppListCommands(component); err != nil {
			e.logger.Error(
				"msg", "AppListCommands() failed",
				"component", component,
				"system_id", e.Client.System.ID,
				"error", err.Error(),
			)
			e.IncrementErrorCounter()
			e.logger.Debug(
				"msg", "GatherMetrics() completed AppListCommands()",
				"component", component,
				"system_id", e.Client.System.ID,
			)
		} else {
			e.IncrementSuccessCounter()
			e.logger.Debug(
				"msg", "GatherMetrics() completed AppListCommands()",
				"component", component,
				"system_id", e.Client.System.ID,
			)
			if cmds["coverage/show"] {
				e.logger.Debug(
					"msg", "GatherMetrics() calls GetAppCoverageMetrics()",
					"component", component,
					"system_id", e.Client.System.ID,
				)
				if metrics, err := e.Client.GetAppCoverageMetrics(component); err != nil {
					e.logger.Error(
						"msg", "GetAppCoverageMetrics() failed",
						"component", component,
						"system_id", e.Client.System.ID,
						"error", err.Error(),
					)
					e.IncrementErrorCounter()
				} else {
					e.IncrementSuccessCounter()
					for event, metric := range metrics {
						//log.Infof("%s: %s, %s", component, name, metric)
						for period, value := range metric {
							if period == "total" {
								e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
									covTotal,
									prometheus.CounterValue,
									value,
									e.Client.System.ID,
									component,
									event,
								))
							} else {
								e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
									covAvg,
									prometheus.GaugeValue,
									value,
									e.Client.System.ID,
									component,
									event,
									period,
								))
							}
						}
					}
				}
				e.logger.Debug(
					"msg", "GatherMetrics() completed GetAppCoverageMetrics()",
					"component", component,
					"system_id", e.Client.System.ID,
				)
			}
			if cmds["memory/show"] {
				e.logger.Debug(
					"msg", "GatherMetrics() calls GetAppMemoryMetrics()",
					"component", component,
					"system_id", e.Client.System.ID,
				)
				if metrics, err := e.Client.GetAppMemoryMetrics(component); err != nil {
					e.logger.Error(
						"msg", "GetAppMemoryMetrics() failed",
						"component", component,
						"system_id", e.Client.System.ID,
						"error", err.Error(),
					)
					e.IncrementErrorCounter()
				} else {
					e.IncrementSuccessCounter()
					for facility, value := range metrics {
						e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
							memUsage,
							prometheus.GaugeValue,
							value,
							e.Client.System.ID,
							component,
							facility,
						))
					}
				}
				e.logger.Debug(
					"msg", "GatherMetrics() completed GetAppMemoryMetrics()",
					"component", component,
					"system_id", e.Client.System.ID,
				)
			}
			if cmds["cluster/status DB"] {
				e.logger.Debug(
					"msg", "GatherMetrics() calls GetAppClusteringInfo()",
					"component", component,
					"system_id", e.Client.System.ID,
				)
				if cluster, err := e.Client.GetAppClusteringInfo(component); err != nil {
					isClusterEnabled = false
					e.logger.Error(
						"msg", "GetAppClusteringInfo() failed",
						"component", component,
						"system_id", e.Client.System.ID,
						"error", err.Error(),
					)
					e.IncrementErrorCounter()
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterEnabled,
						prometheus.GaugeValue,
						0,
						e.Client.System.ID,
						component,
					))
				} else {
					e.IncrementSuccessCounter()
					isClusterEnabled = true
					switch component {
					case "ovsdb-server-southbound":
						southClusterID = cluster.ClusterID
					case "ovsdb-server-northbound":
						northClusterID = cluster.ClusterID
					}
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterEnabled,
						prometheus.GaugeValue,
						1,
						e.Client.System.ID,
						component,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterRole,
						prometheus.GaugeValue,
						float64(cluster.Role),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.UUID,
						cluster.ClusterID,
						cluster.ClusterUUID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterStatus,
						prometheus.GaugeValue,
						float64(cluster.Status),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterTerm,
						prometheus.CounterValue,
						float64(cluster.Term),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterNotCommittedEntryCount,
						prometheus.GaugeValue,
						float64(cluster.NotCommittedEntries),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterNotAppliedEntryCount,
						prometheus.GaugeValue,
						float64(cluster.NotAppliedEntries),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterNextIndex,
						prometheus.CounterValue,
						float64(cluster.NextIndex),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterMatchIndex,
						prometheus.CounterValue,
						float64(cluster.MatchIndex),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterLogLowIndex,
						prometheus.CounterValue,
						float64(cluster.Log.Low),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterLogHighIndex,
						prometheus.CounterValue,
						float64(cluster.Log.High),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterLeaderSelf,
						prometheus.GaugeValue,
						float64(cluster.IsLeaderSelf),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterVoteSelf,
						prometheus.GaugeValue,
						float64(cluster.IsVotedSelf),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterPeerCount,
						prometheus.GaugeValue,
						float64(len(cluster.Peers)),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterPeerInConnTotal,
						prometheus.GaugeValue,
						float64(cluster.Connections.Inbound),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
						clusterPeerOutConnTotal,
						prometheus.GaugeValue,
						float64(cluster.Connections.Outbound),
						e.Client.System.ID,
						component,
						cluster.ID,
						cluster.ClusterID,
					))
					for peerID, peer := range cluster.Peers {
						e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
							clusterPeerNextIndex,
							prometheus.CounterValue,
							float64(peer.NextIndex),
							e.Client.System.ID,
							component,
							cluster.ID,
							cluster.ClusterID,
							peerID,
						))
						e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
							clusterPeerMatchIndex,
							prometheus.CounterValue,
							float64(peer.MatchIndex),
							e.Client.System.ID,
							component,
							cluster.ID,
							cluster.ClusterID,
							peerID,
						))
						e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
							clusterPeerInConnInfo,
							prometheus.GaugeValue,
							float64(peer.Connection.Inbound),
							e.Client.System.ID,
							component,
							cluster.ID,
							cluster.ClusterID,
							peerID,
							peer.Address,
						))
						e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
							clusterPeerOutConnInfo,
							prometheus.GaugeValue,
							float64(peer.Connection.Outbound),
							e.Client.System.ID,
							component,
							cluster.ID,
							cluster.ClusterID,
							peerID,
							peer.Address,
						))
					}
					//log.Infof("%s: %v", component, cluster)
				}
				e.logger.Debug(
					"msg", "GatherMetrics() completed GetAppClusteringInfo()",
					"component", component,
					"system_id", e.Client.System.ID,
				)
			}
		}
	}

	if northClusterID != "" && southClusterID != "" {
		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			clusterGroup,
			prometheus.GaugeValue,
			1,
			e.Client.System.ID,
			northClusterID+southClusterID,
		))
	}

	components = []string{
		"ovsdb-server-southbound",
		"ovsdb-server-northbound",
	}

	for _, component := range components {
		e.logger.Debug(
			"msg", "GatherMetrics() calls IsDefaultPortUp()",
			"component", component,
			"system_id", e.Client.System.ID,
		)
		defaultPortUp, err := e.Client.IsDefaultPortUp(component)
		if err != nil {
			e.logger.Error(
				"msg", "IsDefaultPortUp() failed",
				"component", component,
				"system_id", e.Client.System.ID,
				"error", err.Error(),
			)
			e.IncrementErrorCounter()
		} else {
			e.IncrementSuccessCounter()
		}
		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			networkPortUp,
			prometheus.GaugeValue,
			float64(defaultPortUp),
			e.Client.System.ID,
			component,
			"default",
		))
		e.logger.Debug(
			"msg", "GatherMetrics() completed IsDefaultPortUp()",
			"component", component,
			"system_id", e.Client.System.ID,
		)

		e.logger.Debug(
			"msg", "GatherMetrics() calls IsSslPortUp()",
			"component", component,
			"system_id", e.Client.System.ID,
		)
		sslPortUp, err := e.Client.IsSslPortUp(component)
		if err != nil {
			e.logger.Error(
				"msg", "IsSslPortUp() failed",
				"component", component,
				"system_id", e.Client.System.ID,
				"error", err.Error(),
			)
			e.IncrementErrorCounter()
		} else {
			e.IncrementSuccessCounter()
		}
		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			networkPortUp,
			prometheus.GaugeValue,
			float64(sslPortUp),
			e.Client.System.ID,
			component,
			"ssl",
		))
		e.logger.Debug(
			"msg", "GatherMetrics() completed IsSslPortUp()",
			"component", component,
			"system_id", e.Client.System.ID,
		)

		if isClusterEnabled {
			e.logger.Debug(
				"msg", "GatherMetrics() calls IsRaftPortUp()",
				"component", component,
				"system_id", e.Client.System.ID,
			)
			raftPortUp, err := e.Client.IsRaftPortUp(component)
			if err != nil {
				e.logger.Error(
					"msg", "IsRaftPortUp() failed",
					"component", component,
					"system_id", e.Client.System.ID,
					"error", err.Error(),
				)
				e.IncrementErrorCounter()
			} else {
				e.IncrementSuccessCounter()
			}
			e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
				networkPortUp,
				prometheus.GaugeValue,
				float64(raftPortUp),
				e.Client.System.ID,
				component,
				"raft",
			))
			e.logger.Debug(
				"msg", "GatherMetrics() completed IsRaftPortUp()",
				"component", component,
				"system_id", e.Client.System.ID,
			)
		}
	}

	e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
		up,
		prometheus.GaugeValue,
		float64(upValue),
	))

	e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
		info,
		prometheus.GaugeValue,
		1,
		e.Client.System.ID, e.Client.System.Hostname,
		e.Client.System.Type, e.Client.System.Version,
		e.Client.Database.Northbound.Schema.Version, e.Client.Database.Southbound.Schema.Version,
	))

	e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
		requestErrors,
		prometheus.CounterValue,
		float64(e.errors),
		e.Client.System.ID,
	))

	e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
		requestsTotal,
		prometheus.CounterValue,
		float64(e.totalRequests),
		e.Client.System.ID,
	))

	e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
		requestsSuccessful,
		prometheus.CounterValue,
		float64(e.successfulRequests),
		e.Client.System.ID,
	))

	e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
		nextPoll,
		prometheus.CounterValue,
		float64(e.nextCollectionTicker),
		e.Client.System.ID,
	))

	e.nextCollectionTicker = time.Now().Add(time.Duration(e.pollInterval) * time.Second).Unix()

	e.logger.Debug(
		"msg", "GatherMetrics() returns",
		"system_id", e.Client.System.ID,
	)
}

// OvnLogicalRouter holds basic information about a logical router
type OvnLogicalRouter struct {
	UUID         string            `json:"uuid" yaml:"uuid"`
	Name         string            `json:"name" yaml:"name"`
	ExternalIDs  map[string]string `json:"external_ids" yaml:"external_ids"`
	Ports        []string          `json:"ports" yaml:"ports"`
	StaticRoutes []string          `json:"static_routes" yaml:"static_routes"`
	NAT          []string          `json:"nat" yaml:"nat"`
	LoadBalancer []string          `json:"load_balancer" yaml:"load_balancer"`
	Policies     []string          `json:"policies" yaml:"policies"`
}

// GetLogicalRouters returns a list of OVN logical routers from the Northbound database
func (e *Exporter) GetLogicalRouters() ([]*OvnLogicalRouter, error) {
	routers := []*OvnLogicalRouter{}

	// Query the Logical_Router table in the Northbound database
	query := "SELECT _uuid, external_ids, name, ports, static_routes, nat, load_balancer, policies FROM Logical_Router"
	result, err := e.Client.Database.Northbound.Client.Transact(e.Client.Database.Northbound.Name, query)
	if err != nil {
		return nil, fmt.Errorf("%s: 'Logical_Router' table error: %s", e.Client.Database.Northbound.Name, err)
	}

	if len(result.Rows) == 0 {
		// No routers found is not an error, just return empty list
		return routers, nil
	}

	for _, row := range result.Rows {
		router := &OvnLogicalRouter{}

		// Get UUID
		if r, dt, err := row.GetColumnValue("_uuid", result.Columns); err == nil && dt == "string" {
			router.UUID = r.(string)
		} else {
			continue
		}

		// Get Name
		if r, dt, err := row.GetColumnValue("name", result.Columns); err == nil && dt == "string" {
			router.Name = r.(string)
		}

		// Get External IDs
		if r, dt, err := row.GetColumnValue("external_ids", result.Columns); err == nil && dt == "map[string]string" {
			router.ExternalIDs = r.(map[string]string)
		} else {
			router.ExternalIDs = make(map[string]string)
		}

		// Get Ports
		if r, dt, err := row.GetColumnValue("ports", result.Columns); err == nil {
			switch dt {
			case "string":
				router.Ports = append(router.Ports, r.(string))
			case "[]string":
				router.Ports = r.([]string)
			}
		}

		// Get Static Routes
		if r, dt, err := row.GetColumnValue("static_routes", result.Columns); err == nil {
			switch dt {
			case "string":
				router.StaticRoutes = append(router.StaticRoutes, r.(string))
			case "[]string":
				router.StaticRoutes = r.([]string)
			}
		}

		// Get NAT rules
		if r, dt, err := row.GetColumnValue("nat", result.Columns); err == nil {
			switch dt {
			case "string":
				router.NAT = append(router.NAT, r.(string))
			case "[]string":
				router.NAT = r.([]string)
			}
		}

		// Get Load Balancers
		if r, dt, err := row.GetColumnValue("load_balancer", result.Columns); err == nil {
			switch dt {
			case "string":
				router.LoadBalancer = append(router.LoadBalancer, r.(string))
			case "[]string":
				router.LoadBalancer = r.([]string)
			}
		}

		// Get Policies
		if r, dt, err := row.GetColumnValue("policies", result.Columns); err == nil {
			switch dt {
			case "string":
				router.Policies = append(router.Policies, r.(string))
			case "[]string":
				router.Policies = r.([]string)
			}
		}

		routers = append(routers, router)
	}

	return routers, nil
}

func init() {
	// Note: version.NewCollector has been removed in newer prometheus/common versions
	// The build info is now exposed via the standard prometheus collector
}

// GetVersionInfo returns exporter info.
func GetVersionInfo() string {
	return version.Info()
}

// GetVersionBuildContext returns exporter build context.
func GetVersionBuildContext() string {
	return version.BuildContext()
}

// GetVersion returns exporter version.
func GetVersion() string {
	return version.Version
}

// GetRevision returns exporter revision.
func GetRevision() string {
	return version.Revision
}

// GetExporterName returns exporter name.
func GetExporterName() string {
	return app.Name
}

// GetExporterVersion return full version info.
func GetExporterVersion() string {
	return app.Banner()
}

// SetPollInterval sets exporter's polling interval.
func (e *Exporter) SetPollInterval(i int64) {
	e.pollInterval = i
}
