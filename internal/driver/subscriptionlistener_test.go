// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"testing"

	"github.com/edgexfoundry/device-opcua-go/internal/config"
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/interfaces/mocks"
	sdkModels "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func Test_startSubscriptionListener(t *testing.T) {
	t.Run("create context and exit", func(t *testing.T) {

		deviceName := "Test"
		protocols := make(map[string]models.ProtocolProperties)
		keyMap := make(map[string]any)
		keyMap["Endpoint"] = "opc.tcp://test.com:50551"
		keyMap["Mode"] = "None"
		keyMap["Policy"] = "None"
		keyMap["KeyFile"] = ""
		keyMap["CertFile"] = ""
		protocols["opcua"] = keyMap
		device := models.Device{Name: "Test", ProfileName: "Test", Protocols: protocols}

		_, cancel := context.WithCancel(context.Background())

		d := NewProtocolDriver().(*Driver)
		asyncValues := make(chan *sdkModels.AsyncValues)
		service := &mocks.DeviceServiceSDK{}
		service.On("GetDeviceByName", deviceName).Return(device, nil)
		service.On("LoggingClient").Return(logger.NewMockClient())
		service.On("AsyncValuesChannel").Return(asyncValues)
		d.serviceConfig = &config.ServiceConfig{}
		d.sdk = service
		d.serviceConfig.OPCUAServer.Writable.Resources = "IntVarTest1"
		d.ctxCancel = cancel

		err := d.startSubscriptionListener(deviceName)
		if err == nil {
			t.Error("expected err to exist in test environment")
		}

		d.ctxCancel()
	})
}

func Test_onIncomingDataListener(t *testing.T) {
	t.Run("set reading and exit", func(t *testing.T) {
		d := NewProtocolDriver().(*Driver)
		d.serviceConfig = &config.ServiceConfig{}
		asyncValues := make(chan *sdkModels.AsyncValues)
		deviceName := "Test"
		service := &mocks.DeviceServiceSDK{}
		keyMap := make(map[string]any)
		keyMap["nodeId"] = "ns=3;i=1001"
		minValue := new(float64)
		*minValue = 0
		maxValue := new(float64)
		*maxValue = 30

		tdr := models.DeviceResource{
			Description: "test device resource",
			Name:        "TestResource",
			IsHidden:    false,
			Properties: models.ResourceProperties{
				ValueType:    "Int32",
				ReadWrite:    "R",
				Minimum:      minValue,
				Maximum:      maxValue,
				DefaultValue: "0",
			},
			Attributes: keyMap,
		}
		d.Logger = logger.NewMockClient()
		service.On("DeviceResource", deviceName, "TestResource").Return(tdr, true)
		service.On("AsyncValuesChannel").Return(asyncValues)
		d.sdk = service
		err := d.onIncomingDataReceived("42", "TestResource", deviceName)
		if err == nil {
			t.Error("expected err to exist in test environment")
		}
	})
}

func TestDriver_getClient(t *testing.T) {
	tests := []struct {
		name          string
		serviceConfig *config.ServiceConfig
		device        models.Device
		want          *opcua.Client
		wantErr       bool
	}{
		{
			name:          "NOK - no endpoint configured",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			device: models.Device{
				Protocols: make(map[string]models.ProtocolProperties),
			},
			wantErr: true,
		},
		{
			name:          "NOK - no server connection",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			device: models.Device{
				Protocols: map[string]models.ProtocolProperties{
					"opcua": {"Endpoint": "opc.tcp://test"},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewProtocolDriver().(*Driver)
			d.serviceConfig = &config.ServiceConfig{}
			_, err := d.getClient(context.Background(), tt.device)
			if (err != nil) != tt.wantErr {
				t.Errorf("Driver.getClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDriver_handleDataChange(t *testing.T) {
	tests := []struct {
		name        string
		resourceMap map[uint32]string
		dcn         *ua.DataChangeNotification
	}{
		{
			name: "OK - no monitored items",
			dcn:  &ua.DataChangeNotification{MonitoredItems: make([]*ua.MonitoredItemNotification, 0)},
		},
		{
			name:        "OK - call onIncomingDataReceived",
			resourceMap: map[uint32]string{123456: "TestResource"},
			dcn: &ua.DataChangeNotification{
				MonitoredItems: []*ua.MonitoredItemNotification{
					{ClientHandle: 123456, Value: &ua.DataValue{Value: ua.MustVariant("42")}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewProtocolDriver().(*Driver)
			d.serviceConfig = &config.ServiceConfig{}
			deviceName := "Test"
			service := &mocks.DeviceServiceSDK{}
			keyMap := make(map[string]any)
			keyMap["nodeId"] = "ns=3;i=1001"
			minValue := new(float64)
			*minValue = 0
			maxValue := new(float64)
			*maxValue = 30

			tdr := models.DeviceResource{
				Description: "test device resource",
				Name:        "TestResource",
				IsHidden:    false,
				Properties: models.ResourceProperties{
					ValueType:    "Int32",
					ReadWrite:    "R",
					Minimum:      minValue,
					Maximum:      maxValue,
					DefaultValue: "0",
				},
				Attributes: keyMap,
			}
			d.Logger = logger.NewMockClient()
			service.On("DeviceResource", deviceName, "TestResource").Return(tdr, true)
			d.sdk = service
			d.resourceMap = tt.resourceMap
			subscriptionMap := make(map[uint32]string)
			subscriptionMap[432] = "Test"
			d.deviceMap = subscriptionMap
			d.handleDataChange(tt.dcn, 432)
		})
	}
}
