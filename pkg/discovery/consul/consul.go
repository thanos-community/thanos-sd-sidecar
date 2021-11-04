// Copyright (c) Thanos Contributors
// Licensed under the Apache License 2.0.

// Taken from Prometheus project.

// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package consul

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/util/strutil"
	"github.com/thanos-io/thanos/pkg/store/storepb"
	"google.golang.org/grpc"
)

var (
	// addressLabel is the name for the label containing a target's address.
	addressLabel = model.MetaLabelPrefix + "consul_address"
	// nodeLabel is the name for the label containing a target's node name.
	nodeLabel = model.MetaLabelPrefix + "consul_node"
	// tagsLabel is the name of the label containing the tags assigned to the target.
	tagsLabel = model.MetaLabelPrefix + "consul_tags"
	// serviceAddressLabel is the name of the label containing the (optional) service address.
	serviceAddressLabel = model.MetaLabelPrefix + "consul_service_address"
	// servicePortLabel is the name of the label containing the service port.
	servicePortLabel = model.MetaLabelPrefix + "consul_service_port"
	// serviceIDLabel is the name of the label containing the service ID.
	serviceIDLabel = model.MetaLabelPrefix + "consul_service_id"
)

// CatalogService is copied from https://github.com/hashicorp/consul/blob/master/api/catalog.go
// this struct represents the response from a /service/<service-name> request.
// Consul License: https://github.com/hashicorp/consul/blob/master/LICENSE
type CatalogService struct {
	ID                       string
	Node                     string
	Address                  string
	Datacenter               string
	TaggedAddresses          map[string]string
	NodeMeta                 map[string]string
	ServiceID                string
	ServiceName              string
	ServiceAddress           string
	ServiceTags              []string
	ServicePort              int
	ServiceEnableTagOverride bool
	CreateIndex              uint64
	ModifyIndex              uint64
}

// Note: create a config struct for your custom SD type here.
type ConsulSDConfig struct {
	Address         string `yaml:"server,omitempty"`
	TagSeparator    string `yaml:"tag_separator,omitempty"`
	RefreshInterval int    `yaml:"refresh_interval,omitempty"`
}

// Note: This is the struct with your implementation of the Discoverer interface (see Run function).
// Discovery retrieves target information from a Consul server and updates them via watches.
type Discovery struct {
	address         string
	refreshInterval int
	tagSeparator    string
	logger          log.Logger
	oldSourceList   map[string]bool
}

type EndpointClients struct {
	store storepb.StoreClient
}

// Check if given service is Related to Thanos.
func (d *Discovery) checkIfStore(ctx context.Context, addr string) (bool, error) {
	level.Info(d.logger).Log("msg", "InfoRequest to endpoint", "addr", addr)

	conn, err := grpc.DialContext(ctx, addr, grpc.WithInsecure(), grpc.WithDefaultCallOptions())
	if err != nil {
		level.Warn(d.logger).Log("msg", "Error grpc.DialContext()", "err", err)
		return false, err
	}

	ec := &EndpointClients{
		store: storepb.NewStoreClient(conn),
	}
	in := &storepb.InfoRequest{}
	_, err = ec.store.Info(ctx, in)
	if err != nil {
		level.Warn(d.logger).Log("msg", "Error store.Info()", "err", err)
		return false, err
	}

	return true, nil
}

func (d *Discovery) parseServiceNodes(ctx context.Context, resp *http.Response, name string) (*targetgroup.Group, error) {
	var nodes []*CatalogService
	tgroup := targetgroup.Group{
		Source: name,
		Labels: make(model.LabelSet),
	}

	defer func() {
		_, err := io.Copy(ioutil.Discard, resp.Body)
		if err != nil {
			level.Warn(d.logger).Log("msg", "Error io.Copy()", "err", err)
		}
		resp.Body.Close()
	}()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &nodes)
	if err != nil {
		return &tgroup, err
	}

	tgroup.Targets = make([]model.LabelSet, 0)

	for _, node := range nodes {
		// We surround the separated list with the separator as well. This way regular expressions
		// in relabeling rules don't have to consider tag positions.
		tags := "," + strings.Join(node.ServiceTags, ",") + ","

		// If the service address is not empty it should be used instead of the node address
		// since the service may be registered remotely through a different node.
		var addr string
		if node.ServiceAddress != "" {
			addr = net.JoinHostPort(node.ServiceAddress, fmt.Sprintf("%d", node.ServicePort))
		} else {
			addr = net.JoinHostPort(node.Address, fmt.Sprintf("%d", node.ServicePort))
		}

		// Node does not expose Thanos StoreAPI, so don't append Target.
		if ok, err := d.checkIfStore(ctx, addr); !ok || err != nil {
			continue
		}

		target := model.LabelSet{model.AddressLabel: model.LabelValue(addr)}
		labels := model.LabelSet{
			model.AddressLabel:                   model.LabelValue(addr),
			model.LabelName(addressLabel):        model.LabelValue(node.Address),
			model.LabelName(nodeLabel):           model.LabelValue(node.Node),
			model.LabelName(tagsLabel):           model.LabelValue(tags),
			model.LabelName(serviceAddressLabel): model.LabelValue(node.ServiceAddress),
			model.LabelName(servicePortLabel):    model.LabelValue(strconv.Itoa(node.ServicePort)),
			model.LabelName(serviceIDLabel):      model.LabelValue(node.ServiceID),
		}
		tgroup.Labels = labels

		// Add all key/value pairs from the node's metadata as their own labels.
		for k, v := range node.NodeMeta {
			name := strutil.SanitizeLabelName(k)
			tgroup.Labels[model.LabelName(model.MetaLabelPrefix+name)] = model.LabelValue(v)
		}
		tgroup.Targets = append(tgroup.Targets, target)
	}

	// No Thanos targets found, so no need to return target group.
	if len(tgroup.Targets) == 0 {
		return nil, nil
	}
	level.Info(d.logger).Log("msg", "Parsed target group", "source", name, "targets", len(tgroup.Targets))

	return &tgroup, nil
}

// Note: you must implement this function for your discovery implementation as part of the
// Discoverer interface. Here you should query your SD for it's list of known targets, determine
// which of those targets you care about (for example, which of Consuls known services do you want
// to scrape for metrics), and then send those targets as a target.TargetGroup to the ch channel.
func (d *Discovery) Run(ctx context.Context, ch chan<- []*targetgroup.Group) {
	for c := time.NewTicker(time.Duration(d.refreshInterval) * time.Second); ; {
		var srvs map[string][]string
		level.Info(d.logger).Log("msg", "Getting services list from Consul via HTTP API")

		resp, err := http.Get(fmt.Sprintf("http://%s/v1/catalog/services", d.address))
		if err != nil {
			level.Error(d.logger).Log("msg", "Error getting services list", "err", err)
			time.Sleep(time.Duration(d.refreshInterval) * time.Second)
			continue
		}

		b, err := ioutil.ReadAll(resp.Body)
		_, errCopy := io.Copy(ioutil.Discard, resp.Body)
		if errCopy != nil {
			level.Warn(d.logger).Log("msg", "Error io.Copy()", "err", errCopy)
		}

		resp.Body.Close()
		if err != nil {
			level.Error(d.logger).Log("msg", "Error reading services list", "err", err)
			time.Sleep(time.Duration(d.refreshInterval) * time.Second)
			continue
		}

		err = json.Unmarshal(b, &srvs)
		resp.Body.Close()
		if err != nil {
			level.Error(d.logger).Log("msg", "Error parsing services list", "err", err)
			time.Sleep(time.Duration(d.refreshInterval) * time.Second)
			continue
		}

		var tgs []*targetgroup.Group
		// Note that we treat errors when querying specific consul services as fatal for this
		// iteration of the time.Tick loop. It's better to have some stale targets than an incomplete
		// list of targets simply because there may have been a timeout. If the service is actually
		// gone as far as consul is concerned, that will be picked up during the next iteration of
		// the outer loop.

		newSourceList := make(map[string]bool)
		for name := range srvs {
			if name == "consul" {
				continue
			}
			level.Info(d.logger).Log("msg", "Getting services nodes", "service", name)

			resp, err := http.Get(fmt.Sprintf("http://%s/v1/catalog/service/%s", d.address, name))
			if err != nil {
				level.Error(d.logger).Log("msg", "Error getting services nodes", "service", name, "err", err)
				break
			}

			tg, err := d.parseServiceNodes(ctx, resp, name)
			if err != nil {
				level.Error(d.logger).Log("msg", "Error parsing services nodes", "service", name, "err", err)
				break
			}
			// No target group returned.
			if tg != nil {
				tgs = append(tgs, tg)
				newSourceList[tg.Source] = true
			}
		}
		// When targetGroup disappear, send an update with empty targetList.
		for key := range d.oldSourceList {
			if !newSourceList[key] {
				tgs = append(tgs, &targetgroup.Group{
					Source: key,
				})
			}
		}
		d.oldSourceList = newSourceList
		if err == nil {
			// We're returning all Consul services as a single targetgroup.
			ch <- tgs
		}
		// Wait for ticker or exit when ctx is closed.
		select {
		case <-c.C:
			continue
		case <-ctx.Done():
			return
		}
	}
}

func NewDiscovery(conf ConsulSDConfig, logger log.Logger) (*Discovery, error) {
	cd := &Discovery{
		address:         conf.Address,
		refreshInterval: conf.RefreshInterval,
		tagSeparator:    conf.TagSeparator,
		logger:          logger,
		oldSourceList:   make(map[string]bool),
	}
	return cd, nil
}
