# Thanos Service Discovery Sidecar

A service discovery sidecar which uses all Prometheus Discovery implementations and generated `file_sd` output compatible with Thanos.

```bash mdox-exec="thanos-sd-sidecar --help"
usage: thanos-sd-sidecar [<flags>] <command> [<args> ...]

Thanos Service Discovery Sidecar.

Flags:
  -h, --help               Show context-sensitive help (also try --help-long and
                           --help-man).
      --version            Show application version.
      --log.level=info     Log filtering level.
      --log.format=clilog  Log format to use.

Commands:
  help [<command>...]
    Show help.

  run --output.path=OUTPUT.PATH [<flags>]
    Launches sidecar for Thanos Service Discovery which generates file_sd output
    (https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config)
    according to configuration.


```
