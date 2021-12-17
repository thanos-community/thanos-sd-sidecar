// Copyright (c) Thanos Contributors
// Licensed under the Apache License 2.0.

package discovery

import (
	"bytes"

	"github.com/pkg/errors"
	"github.com/thanos-community/thanos-sd-sidecar/pkg/discovery/consul"
	"gopkg.in/yaml.v3"
)

// SDConfig with fields for all implementations.
type SDConfig struct {
	ConsulSDConfig consul.ConsulSDConfig `yaml:"consul_sd_config,omitempty"`
}

func ParseConfig(c []byte) (SDConfig, error) {
	cfg := SDConfig{}
	dec := yaml.NewDecoder(bytes.NewReader(c))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return SDConfig{}, errors.Wrapf(err, "parsing YAML content %q", string(c))
	}

	return cfg, nil
}
