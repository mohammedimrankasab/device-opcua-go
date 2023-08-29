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

	sdkModel "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific device resource (aka DeviceObject).
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (d *Driver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties,
	reqs []sdkModel.CommandRequest, params []*sdkModel.CommandValue) error {

	d.Logger.Debugf("Driver.HandleWriteCommands: protocols: %v, resource: %v, parameters: %v", protocols, reqs[0].DeviceResourceName, params)
	var err error

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := d.getClientByProtocols(ctx, protocols)
	if err != nil {
		return err
	}
	defer client.Close(ctx)
	if err := client.Connect(ctx); err != nil {
		d.Logger.Warnf("Driver.HandleWriteCommands: Failed to connect OPCUA client, %s", err)
		return err
	}
	defer client.Close(ctx)

	return d.processWriteCommands(client, reqs, params)
}

func (d *Driver) processWriteCommands(client *opcua.Client, reqs []sdkModel.CommandRequest, params []*sdkModel.CommandValue) error {
	for i, req := range reqs {
		err := d.handleWriteCommandRequest(client, req, params[i])
		if err != nil {
			d.Logger.Errorf("Driver.HandleWriteCommands: Handle write commands failed: %v", err)
			return err
		}
	}

	return nil
}

func (d *Driver) handleWriteCommandRequest(deviceClient *opcua.Client, req sdkModel.CommandRequest,
	param *sdkModel.CommandValue) error {
	nodeID, err := getNodeID(req.Attributes, NODE)
	if err != nil {
		return fmt.Errorf("Driver.handleWriteCommands: %v", err)
	}

	// get NewNodeID
	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("Driver.handleWriteCommands: Invalid node id=%s", nodeID)
	}

	value, err := newCommandValue(req.Type, param)
	if err != nil {
		return err
	}

	v, err := ua.NewVariant(value)
	if err != nil {
		return fmt.Errorf("Driver.handleWriteCommands: invalid value: %v", err)
	}

	request := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      id,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					EncodingMask: ua.DataValueValue, // encoding mask
					Value:        v,
				},
			},
		},
	}

	timeout := 60 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	resp, err := deviceClient.Write(ctx, request)
	if err != nil {
		d.Logger.Errorf("Driver.handleWriteCommands: Write value %v failed: %s", v, err)
		return err
	}
	d.Logger.Infof("Driver.handleWriteCommands: write successfully, %v", resp.Results[0])
	return nil
}

func newCommandValue(valueType string, param *sdkModel.CommandValue) (interface{}, error) {
	var commandValue interface{}
	var err error
	switch valueType {
	case common.ValueTypeBool:
		commandValue, err = param.BoolValue()
	case common.ValueTypeString:
		commandValue, err = param.StringValue()
	case common.ValueTypeUint8:
		commandValue, err = param.Uint8Value()
	case common.ValueTypeUint16:
		commandValue, err = param.Uint16Value()
	case common.ValueTypeUint32:
		commandValue, err = param.Uint32Value()
	case common.ValueTypeUint64:
		commandValue, err = param.Uint64Value()
	case common.ValueTypeInt8:
		commandValue, err = param.Int8Value()
	case common.ValueTypeInt16:
		commandValue, err = param.Int16Value()
	case common.ValueTypeInt32:
		commandValue, err = param.Int32Value()
	case common.ValueTypeInt64:
		commandValue, err = param.Int64Value()
	case common.ValueTypeFloat32:
		commandValue, err = param.Float32Value()
	case common.ValueTypeFloat64:
		commandValue, err = param.Float64Value()
	default:
		err = fmt.Errorf("fail to convert param, none supported value type: %v", valueType)
	}

	return commandValue, err
}
