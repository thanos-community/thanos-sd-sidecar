# Thanos Service Discovery Sidecar

A service discovery sidecar which uses all Prometheus Discovery implementations and generates `file_sd` output compatible with Thanos.

```bash mdox-exec="thanos-sd-sidecar run --help"
usage: thanos-sd-sidecar run [<flags>]

Launches sidecar for Thanos Service Discovery which generates file_sd output
(https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config)
according to configuration.

Flags:
  -h, --help                     Show context-sensitive help (also try
                                 --help-long and --help-man).
      --version                  Show application version.
      --log.level=info           Log filtering level.
      --log.format=clilog        Log format to use.
      --config-file=<file-path>  Path to YAML file for Service Discovery
                                 configuration, with spec defined in
                                 https://prometheus.io/docs/prometheus/latest/configuration/configuration.
      --config=<content>         Alternative to 'config-file' flag (mutually
                                 exclusive). Content of YAML file for Service
                                 Discovery configuration, with spec defined in
                                 https://prometheus.io/docs/prometheus/latest/configuration/configuration.
      --output.path="targets.json"  
                                 The output path for file_sd compatible files.
      --http.sd                  Enable service discovery endpoint (/targets)
                                 which serves SD targets compatible with
                                 https://prometheus.io/docs/prometheus/latest/http_sd.

```
