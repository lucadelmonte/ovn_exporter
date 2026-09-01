package main

import (
	"net/http"
	"os"

	ovn "github.com/Liquescent-Development/ovn_exporter/pkg/ovn_exporter"
	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	prometheus_version "github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

var version string

func main() {

	prometheus_version.Version = version

	var (
		pollTimeout                     = kingpin.Flag("ovn.timeout", "Timeout on gRPC requests to OVN.").Default("2").Int()
		pollInterval                    = kingpin.Flag("ovn.poll-interval", "The minimum interval (in seconds) between collections from OVN server.").Default("15").Int()
		databaseNorthboundName          = kingpin.Flag("database.northbound.name", "The name of OVN NB (northbound) db.").Default("OVN_Northbound").String()
		databaseNorthboundSocketRemote  = kingpin.Flag("database.northbound.socket.remote", "JSON-RPC unix socket to OVN NB db.").Default("unix:/run/openvswitch/ovnnb_db.sock").String()
		databaseNorthboundSocketControl = kingpin.Flag("database.northbound.socket.control", "JSON-RPC unix socket to OVN NB app.").Default("unix:/run/openvswitch/ovnnb_db.ctl").String()
		databaseNorthboundFileDataPath  = kingpin.Flag("database.northbound.file.data.path", "OVN NB db file.").Default("/var/lib/openvswitch/ovnnb_db.db").String()
		databaseNorthboundFileLogPath   = kingpin.Flag("database.northbound.file.log.path", "OVN NB db log file.").Default("/var/log/openvswitch/ovsdb-server-nb.log").String()
		databaseNorthboundFilePidPath   = kingpin.Flag("database.northbound.file.pid.path", "OVN NB db process id file.").Default("/run/openvswitch/ovnnb_db.pid").String()
		databaseNorthboundPortDefault   = kingpin.Flag("database.northbound.port.default", "OVN NB db network socket port.").Default("6641").Int()
		databaseNorthboundPortSsl       = kingpin.Flag("database.northbound.port.ssl", "OVN NB db network socket secure port.").Default("6631").Int()
		databaseNorthboundPortRaft      = kingpin.Flag("database.northbound.port.raft", "OVN NB db network port for clustering (raft)").Default("6643").Int()

		databaseSouthboundName          = kingpin.Flag("database.southbound.name", "The name of OVN SB (southbound) db.").Default("OVN_Southbound").String()
		databaseSouthboundSocketRemote  = kingpin.Flag("database.southbound.socket.remote", "JSON-RPC unix socket to OVN SB db.").Default("unix:/run/openvswitch/ovnsb_db.sock").String()
		databaseSouthboundSocketControl = kingpin.Flag("database.southbound.socket.control", "JSON-RPC unix socket to OVN SB app.").Default("unix:/run/openvswitch/ovnsb_db.ctl").String()
		databaseSouthboundFileDataPath  = kingpin.Flag("database.southbound.file.data.path", "OVN SB db file.").Default("/var/lib/openvswitch/ovnsb_db.db").String()
		databaseSouthboundFileLogPath   = kingpin.Flag("database.southbound.file.log.path", "OVN SB db log file.").Default("/var/log/openvswitch/ovsdb-server-sb.log").String()
		databaseSouthboundFilePidPath   = kingpin.Flag("database.southbound.file.pid.path", "OVN SB db process id file.").Default("/run/openvswitch/ovnsb_db.pid").String()
		databaseSouthboundPortDefault   = kingpin.Flag("database.southbound.port.default", "OVN SB db network socket port.").Default("6642").Int()
		databaseSouthboundPortSsl       = kingpin.Flag("database.southbound.port.ssl", "OVN SB db network socket secure port.").Default("6632").Int()
		databaseSouthboundPortRaft      = kingpin.Flag("database.southbound.port.raft", "OVN SB db network port for clustering (raft)").Default("6644").Int()

		serviceNorthdFileLogPath   = kingpin.Flag("service.ovn.northd.file.log.path", "OVN northd daemon log file.").Default("/var/log/openvswitch/ovn-northd.log").String()
		serviceNorthdFilePidPath   = kingpin.Flag("service.ovn.northd.file.pid.path", "OVN northd daemon process id file.").Default("/run/openvswitch/ovn-northd.pid").String()
		serviceNorthdSocketControl = kingpin.Flag("service.ovn.northd.socket.control", "JSON-RPC unix socket to OVN northd app.").Default("unix:/run/openvswitch/ovn-northd.ctl").String()
	)

	metricsPath := kingpin.Flag(
		"web.telemetry-path", "Path under which to expose metrics",
	).Default("/metrics").String()

	toolkitFlags := webflag.AddFlags(kingpin.CommandLine, ":9476")

	promlogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(prometheus_version.Print("ovn_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promslog.New(promlogConfig)

	opts := ovn.Options{
		Timeout: *pollTimeout,
		Logger:  logger,
	}

	exporter, err := ovn.NewExporter(opts)
	if err != nil {
		logger.Error(
			"msg", "failed to init properly",
			"error", err.Error(),
		)
		os.Exit(1)
	}

	exporter.Client.Database.Northbound.Name = *databaseNorthboundName
	exporter.Client.Database.Northbound.Socket.Remote = *databaseNorthboundSocketRemote
	exporter.Client.Database.Northbound.Socket.Control = *databaseNorthboundSocketControl
	exporter.Client.Database.Northbound.File.Data.Path = *databaseNorthboundFileDataPath
	exporter.Client.Database.Northbound.File.Log.Path = *databaseNorthboundFileLogPath
	exporter.Client.Database.Northbound.File.Pid.Path = *databaseNorthboundFilePidPath
	exporter.Client.Database.Northbound.Port.Default = *databaseNorthboundPortDefault
	exporter.Client.Database.Northbound.Port.Ssl = *databaseNorthboundPortSsl
	exporter.Client.Database.Northbound.Port.Raft = *databaseNorthboundPortRaft

	exporter.Client.Database.Southbound.Name = *databaseSouthboundName
	exporter.Client.Database.Southbound.Socket.Remote = *databaseSouthboundSocketRemote
	exporter.Client.Database.Southbound.Socket.Control = *databaseSouthboundSocketControl
	exporter.Client.Database.Southbound.File.Data.Path = *databaseSouthboundFileDataPath
	exporter.Client.Database.Southbound.File.Log.Path = *databaseSouthboundFileLogPath
	exporter.Client.Database.Southbound.File.Pid.Path = *databaseSouthboundFilePidPath
	exporter.Client.Database.Southbound.Port.Default = *databaseSouthboundPortDefault
	exporter.Client.Database.Southbound.Port.Ssl = *databaseSouthboundPortSsl
	exporter.Client.Database.Southbound.Port.Raft = *databaseSouthboundPortRaft

	exporter.Client.Service.Northd.File.Log.Path = *serviceNorthdFileLogPath
	exporter.Client.Service.Northd.File.Pid.Path = *serviceNorthdFilePidPath
	exporter.Client.Service.Northd.Socket.Control = *serviceNorthdSocketControl

	exporter, err = ovn.ExporterPerformClientCalls(exporter)
	if err != nil {
		logger.Error(
			"msg", "failed to finalize exporter calls properly",
			"exporter_name", ovn.GetExporterName(),
			"error", err.Error(),
		)
	}

	logger.Info("ovs_system_id", exporter.Client.System.ID)

	exporter.SetPollInterval(int64(*pollInterval))
	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath, promhttp.Handler())
	if *metricsPath != "/" {
		landingCnf := web.LandingConfig{
			Name:        "OVN Exporter",
			Description: "Prometheus OVN Exporter",
			Version:     prometheus_version.Info(),
			Links: []web.LandingLinks{
				{
					Address: *metricsPath,
					Text:    "Metrics",
				},
			},
		}

		landingPage, err := web.NewLandingPage(landingCnf)
		if err != nil {
			logger.Error("Failed to generate landing page", "msg", err)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}

	srv := &http.Server{}
	if err = web.ListenAndServe(srv, toolkitFlags, logger); err != nil {
		logger.Error("Failed to start server", "msg", err)
		os.Exit(1)
	}
}
