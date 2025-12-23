module github.com/Liquescent-Development/ovn_exporter

go 1.24.0

toolchain go1.24.6

require (
	github.com/go-kit/log v0.2.1
	github.com/greenpau/ovsdb v1.0.4
	github.com/greenpau/versioned v1.0.28
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/common v0.66.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/sys v0.36.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
)

replace github.com/greenpau/ovsdb => github.com/lucadelmonte/ovsdb v1.0.5
