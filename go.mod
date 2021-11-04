module github.com/thanos-community/thanos-sd-sidecar

go 1.16

require (
	github.com/bwplotka/mdox v0.9.0
	github.com/efficientgo/tools/extkingpin v0.0.0-20210609125236-d73259166f20
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-kit/log v0.2.0
	github.com/oklog/run v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.30.0
	github.com/prometheus/prometheus v1.8.2-0.20210914090109-37468d88dce8
	github.com/thanos-io/thanos v0.23.1
	google.golang.org/grpc v1.40.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

replace (
	google.golang.org/grpc => google.golang.org/grpc v1.29.1

	k8s.io/api => k8s.io/api v0.20.4
	k8s.io/client-go => k8s.io/client-go v0.20.4
)
