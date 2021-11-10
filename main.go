// Copyright (c) Thanos Contributors
// Licensed under the Apache License 2.0.

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/bwplotka/mdox/pkg/clilog"
	extflag "github.com/efficientgo/tools/extkingpin"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"github.com/thanos-community/thanos-sd-sidecar/pkg/adapter"
	"github.com/thanos-community/thanos-sd-sidecar/pkg/discovery"
	"github.com/thanos-community/thanos-sd-sidecar/pkg/discovery/consul"
	"github.com/thanos-community/thanos-sd-sidecar/pkg/extkingpin"
	"github.com/thanos-community/thanos-sd-sidecar/pkg/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	logFormatLogfmt = "logfmt"
	logFormatJson   = "json"
	logFormatCLILog = "clilog"
)

func setupLogger(logLevel, logFormat string) log.Logger {
	var lvl level.Option
	switch logLevel {
	case "error":
		lvl = level.AllowError()
	case "warn":
		lvl = level.AllowWarn()
	case "info":
		lvl = level.AllowInfo()
	case "debug":
		lvl = level.AllowDebug()
	default:
		panic("unexpected log level")
	}
	switch logFormat {
	case logFormatJson:
		return level.NewFilter(log.NewJSONLogger(log.NewSyncWriter(os.Stderr)), lvl)
	case logFormatLogfmt:
		return level.NewFilter(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), lvl)
	case logFormatCLILog:
		fallthrough
	default:
		return level.NewFilter(clilog.New(log.NewSyncWriter(os.Stderr)), lvl)
	}
}

func main() {
	app := extkingpin.NewApp(kingpin.New(filepath.Base(os.Args[0]), `Thanos Service Discovery Sidecar.`).Version(version.Version))
	logLevel := app.Flag("log.level", "Log filtering level.").
		Default("info").Enum("error", "warn", "info", "debug")
	logFormat := app.Flag("log.format", "Log format to use.").
		Default(logFormatCLILog).Enum(logFormatLogfmt, logFormatJson, logFormatCLILog)

	ctx, cancel := context.WithCancel(context.Background())
	registerCommands(ctx, app)

	cmd, runner := app.Parse()
	logger := setupLogger(*logLevel, *logFormat)

	var g run.Group
	g.Add(func() error {
		return runner(ctx, logger)
	}, func(err error) {
		cancel()
	})

	// Listen for termination signals.
	{
		cancel := make(chan struct{})
		g.Add(func() error {
			return interrupt(logger, cancel)
		}, func(error) {
			close(cancel)
		})
	}

	if err := g.Run(); err != nil {
		if *logLevel == "debug" {
			// Use %+v for github.com/pkg/errors error to print with stack.
			level.Error(logger).Log("err", fmt.Sprintf("%+v", errors.Wrapf(err, "%s command failed", cmd)))
			os.Exit(1)
		}
		level.Error(logger).Log("err", errors.Wrapf(err, "%s command failed", cmd))
		os.Exit(1)
	}
}

func interrupt(logger log.Logger, cancel <-chan struct{}) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-c:
		level.Info(logger).Log("msg", "caught signal. Exiting.", "signal", s)
		return nil
	case <-cancel:
		return errors.New("canceled")
	}
}

func registerCommands(_ context.Context, app *extkingpin.App) {
	cmd := app.Command("run", "Launches sidecar for Thanos Service Discovery which generates file_sd output (https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config) according to configuration.")
	config := extflag.RegisterPathOrContent(cmd, "config", "YAML file for Service Discovery configuration, with spec defined in https://prometheus.io/docs/prometheus/latest/configuration/configuration.", extflag.WithEnvSubstitution(), extflag.WithRequired())
	outputPath := cmd.Flag("output.path", "The output path for file_sd compatible files.").Default("targets.json").String()
	httpSD := cmd.Flag("http.sd", "Enable service discovery endpoint (/targets) which serves SD targets compatible with https://prometheus.io/docs/prometheus/latest/http_sd.").Default("false").Bool()

	cmd.Run(func(ctx context.Context, logger log.Logger) error {
		validateConfig, err := config.Content()
		if err != nil {
			return err
		}

		cfg, err := discovery.ParseConfig(validateConfig)
		if err != nil {
			return err
		}

		// TODO(saswatamcode): Make this generalized for all implementations.
		disc, err := consul.NewDiscovery(cfg.ConsulSDConfig, logger)
		if err != nil {
			return err
		}
		sdAdapter := adapter.NewAdapter(ctx, *outputPath, "outputSD", disc, logger)

		if *httpSD {
			go func() {
				http.HandleFunc("/targets", sdAdapter.ServeHTTP)
				level.Error(logger).Log(http.ListenAndServe(":8000", nil))
			}()
		}

		sdAdapter.Run()

		<-ctx.Done()
		return nil
	})
}
