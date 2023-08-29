// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"fmt"
	"time"

	"github.com/edgexfoundry/device-opcua-go/internal/config"
	sdkModels "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func (d *Driver) startSubscriptionListener(deviceName string) error {
	for _, div := range d.sdk.Devices() {
		d.Logger.Infof("Device name is %s", div.Name)
	}
	// Create a cancelable context for Writable configuration
	ctxBg := context.Background()
	ctx, cancel := context.WithCancel(ctxBg)
	d.ctxCancel = cancel
	device, err := d.sdk.GetDeviceByName(deviceName)
	if err != nil {
		return nil
	}
	client, err := d.getClient(ctx, device)
	if err != nil {
		return err
	}

	if err := client.Connect(ctx); err != nil {
		d.Logger.Warnf("[Incoming listener] Failed to connect OPCUA client, %s", err)
		return err
	}
	defer func() {
		err := client.Close(ctx)
		if err != nil {
			d.Logger.Errorf("error while closing client connection %v", err)
		}
	}()

	//go sub.Run(ctx) // start Publish loop
	notifyCh := make(chan *opcua.PublishNotificationData)
	sub, err := client.Subscribe(ctx,
		&opcua.SubscriptionParameters{
			Interval: time.Duration(500) * time.Millisecond,
		}, notifyCh)
	if err != nil {
		d.Logger.Errorf("error in subscription %v", err)
		return err
	}

	defer func() {
		err := sub.Cancel(ctx)
		if err != nil {
			d.Logger.Errorf("error cancelling subscription %v", err)
		}
	}()
	d.Logger.Infof("Created subscription with id %v", sub.SubscriptionID)
	d.mu.Lock()
	d.deviceMap[sub.SubscriptionID] = device.Name
	d.mu.Unlock()

	deviceProfile, err := d.sdk.GetProfileByName(device.ProfileName)
	if err != nil {
		return err
	}
	resources := deviceProfile.DeviceResources

	// No need to start a subscription if there are no resources to monitor
	if len(resources) == 0 {
		d.Logger.Info("[Incoming listener] No resources defined to generate subscriptions.")
		return nil
	}
	if err := d.configureMonitoredItems(ctx, sub, resources, d.deviceMap[sub.SubscriptionID]); err != nil {
		return err
	}

	// read from subscription's notification channel until ctx is cancelled
	for {
		select {
		// context return
		case <-ctx.Done():
			return nil
			// receive Publish Notification Data
		case res := <-notifyCh:
			if res.Error != nil {
				d.Logger.Debug(res.Error.Error())
				continue
			}
			switch x := res.Value.(type) {
			// result type: DateChange StatusChange
			case *ua.DataChangeNotification:
				d.handleDataChange(x, res.SubscriptionID)
			default:
				d.Logger.Infof("what's this publish result? %T", res.Value)
			}
		default:
			d.Logger.Info("Inside default case nothing to do")
		}
	}
}

func (d *Driver) getClientByProtocols(ctx context.Context, protocols map[string]models.ProtocolProperties) (*opcua.Client, error) {
	configuration, cfgErr := config.FetchOPCUAConnDetails(protocols)
	if cfgErr != nil {
		return nil, cfgErr
	}
	endpoints, err := opcua.GetEndpoints(ctx, configuration.EndPoint)
	if err != nil {
		return nil, err
	}
	ep := opcua.SelectEndpoint(endpoints, configuration.Policy, ua.MessageSecurityModeFromString(configuration.Mode))
	if ep == nil {
		return nil, fmt.Errorf("[Incoming listener] Failed to find suitable endpoint")
	}
	ep.EndpointURL = configuration.EndPoint

	opts := []opcua.Option{
		opcua.SecurityPolicy(configuration.Policy),
		opcua.SecurityModeString(configuration.Mode),
		opcua.CertificateFile(configuration.CertFile),
		opcua.PrivateKeyFile(configuration.KeyFile),
		opcua.AuthAnonymous(),
		opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeAnonymous),
	}

	return opcua.NewClient(ep.EndpointURL, opts...), nil
}
func (d *Driver) getClient(ctx context.Context, device models.Device) (*opcua.Client, error) {
	//var (
	//	policy   = d.serviceConfig.OPCUAServer.Policy
	//	mode     = d.serviceConfig.OPCUAServer.Mode
	//	certFile = d.serviceConfig.OPCUAServer.CertFile
	//	keyFile  = d.serviceConfig.OPCUAServer.KeyFile
	//)

	//endpoint, myErr := config.FetchEndpoint(device.Protocols)
	//if myErr != nil {
	//	return nil, myErr
	//}

	configuration, cfgErr := config.FetchOPCUAConnDetails(device.Protocols)
	if cfgErr != nil {
		return nil, cfgErr
	}
	endpoints, err := opcua.GetEndpoints(ctx, configuration.EndPoint)
	if err != nil {
		return nil, err
	}
	ep := opcua.SelectEndpoint(endpoints, configuration.Policy, ua.MessageSecurityModeFromString(configuration.Mode))
	if ep == nil {
		return nil, fmt.Errorf("[Incoming listener] Failed to find suitable endpoint")
	}
	ep.EndpointURL = configuration.EndPoint

	opts := []opcua.Option{
		opcua.SecurityPolicy(configuration.Policy),
		opcua.SecurityModeString(configuration.Mode),
		opcua.CertificateFile(configuration.CertFile),
		opcua.PrivateKeyFile(configuration.KeyFile),
		opcua.AuthAnonymous(),
		opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeAnonymous),
	}

	return opcua.NewClient(ep.EndpointURL, opts...), nil
}

func (d *Driver) configureMonitoredItems(ctx context.Context, sub *opcua.Subscription, resources []models.DeviceResource, deviceName string) error {

	d.mu.Lock()
	defer d.mu.Unlock()

	for i, resource := range resources {
		resourceName := resource.Name
		deviceResource, ok := d.sdk.DeviceResource(deviceName, resourceName)
		if !ok {
			return fmt.Errorf("[Incoming listener] Unable to find device resource with name %s", resourceName)
		}

		opcuaNodeID, err := getNodeID(deviceResource.Attributes, NODE)
		if err != nil {
			return err
		}

		id, err := ua.ParseNodeID(opcuaNodeID)
		if err != nil {
			return err
		}

		// arbitrary client handle for the monitoring item
		handle := uint32(i + 42)
		// map the client handle so we know what the value returned represents
		d.resourceMap[handle] = resourceName
		miCreateRequest := opcua.NewMonitoredItemCreateRequestWithDefaults(id, ua.AttributeIDValue, handle)
		res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, miCreateRequest)
		if err != nil || res.Results[0].StatusCode != ua.StatusOK {
			return err
		}

		d.Logger.Infof("[Incoming listener] Start incoming data listening for %s.", resourceName)
	}

	return nil
}

func (d *Driver) handleDataChange(dcn *ua.DataChangeNotification, subscriptionId uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Logger.Infof("handler data change for subscription ID %d", subscriptionId)
	for _, item := range dcn.MonitoredItems {
		data := item.Value.Value.Value()
		nodeName := d.resourceMap[item.ClientHandle]
		fmt.Println("Going inside the incoming")
		if err := d.onIncomingDataReceived(data, nodeName, d.deviceMap[subscriptionId]); err != nil {
			d.Logger.Errorf("%v", err)
		}
	}
}

func (d *Driver) onIncomingDataReceived(data interface{}, nodeResourceName, deviceName string) error {
	//deviceName := d.serviceConfig.OPCUAServer.DeviceName
	reading := data
	for _, div := range d.sdk.Devices() {
		d.Logger.Infof("Device name in onIncomingDataReceived is %s", div.Name)
	}
	deviceResource, ok := d.sdk.DeviceResource(deviceName, nodeResourceName)
	if !ok {
		d.Logger.Warnf("[Incoming listener] Incoming reading ignored. No DeviceObject found: name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)
		return nil
	}

	req := sdkModels.CommandRequest{
		DeviceResourceName: nodeResourceName,
		Type:               deviceResource.Properties.ValueType,
	}

	result, err := newResult(req, reading)
	if err != nil {
		d.Logger.Warnf("[Incoming listener] Incoming reading ignored. name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)
		return nil
	}

	asyncValues := &sdkModels.AsyncValues{
		DeviceName:    deviceName,
		CommandValues: []*sdkModels.CommandValue{result},
	}
	d.Logger.Infof("[Incoming listener] Incoming reading received: name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)

	d.AsyncCh <- asyncValues
	return nil
}
