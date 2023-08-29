// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"

	"github.com/edgexfoundry/go-mod-core-contracts/v3/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
)

// ServiceConfig configuration struct
type ServiceConfig struct {
	OPCUAServer     OPCUAServerConfig
	OPCUAServerConf OPCUAServerConf
}

// UpdateFromRaw updates the service's full configuration from raw data received from
// the Service Provider.
func (sw *ServiceConfig) UpdateFromRaw(rawConfig interface{}) bool {
	configuration, ok := rawConfig.(*ServiceConfig)
	if !ok {
		return false
	}

	*sw = *configuration

	return true
}

// OPCUAServerConfig server information defined by the device profile
type OPCUAServerConfig struct {
	DeviceName string
	Policy     string
	Mode       string
	CertFile   string
	KeyFile    string
	Writable   WritableInfo
}

type OPCUAServerConf struct {
	EndPoint string
	Policy   string
	Mode     string
	CertFile string
	KeyFile  string
}

// WritableInfo configuration data that can be written without restarting the service
type WritableInfo struct {
	Resources string
}

var policies map[string]int = map[string]int{
	"None":           1,
	"Basic128Rsa15":  2,
	"Basic256":       3,
	"Basic256Sha256": 4,
}

var modes map[string]int = map[string]int{
	"None":           1,
	"Sign":           2,
	"SignAndEncrypt": 3,
}

func (opc *OPCUAServerConf) ValidateConf() errors.EdgeX {
	if opc.EndPoint == "" {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.DeviceName configuration setting cannot be blank", nil)
	}

	if _, ok := policies[opc.Policy]; !ok {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.Policy configuration setting mismatch", nil)
	}
	if _, ok := modes[opc.Mode]; !ok {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.Mode configuration setting mismatch", nil)
	}
	if opc.Mode != "None" || opc.Policy != "None" {
		if opc.CertFile == "" {
			return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.CertFile configuration setting cannot be blank when a security mode or policy is set", nil)
		}
		if opc.KeyFile == "" {
			return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.KeyFile configuration setting cannot be blank when a security mode or policy is set", nil)
		}
	}

	return nil
}

// Validate ensures your custom configuration has proper values.
func (info *OPCUAServerConfig) Validate() errors.EdgeX {
	if info.DeviceName == "" {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.DeviceName configuration setting cannot be blank", nil)
	}

	if _, ok := policies[info.Policy]; !ok {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.Policy configuration setting mismatch", nil)
	}
	if _, ok := modes[info.Mode]; !ok {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.Mode configuration setting mismatch", nil)
	}
	if info.Mode != "None" || info.Policy != "None" {
		if info.CertFile == "" {
			return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.CertFile configuration setting cannot be blank when a security mode or policy is set", nil)
		}
		if info.KeyFile == "" {
			return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.KeyFile configuration setting cannot be blank when a security mode or policy is set", nil)
		}
	}

	return nil
}

func FetchOPCUAConnDetails(protocols map[string]models.ProtocolProperties) (*OPCUAServerConf, errors.EdgeX) {
	properties, ok := protocols[Protocol]
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' protocol properties is not defined", Protocol), nil)
	}
	endpoint, ok := properties[Endpoint]
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' not found in the '%s' protocol properties", Endpoint, Protocol), nil)
	}

	endpointString, ok := endpoint.(string)
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("cannot convert '%v' to string type", endpoint), nil)
	}

	policy, ok := properties[Policy]
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' not found in the '%s' protocol properties", Policy, Protocol), nil)
	}

	policyString, ok := policy.(string)
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("cannot convert '%v' to string type", policy), nil)
	}

	mode, ok := properties[Mode]
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' not found in the '%s' protocol properties", Mode, Protocol), nil)
	}

	modeString, ok := mode.(string)
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("cannot convert '%v' to string type", mode), nil)
	}

	certFile, ok := properties[CertFile]
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' not found in the '%s' protocol properties", CertFile, Protocol), nil)
	}

	certFileString, ok := certFile.(string)
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("cannot convert '%v' to string type", certFile), nil)
	}

	keyFile, ok := properties[KeyFile]
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' not found in the '%s' protocol properties", KeyFile, Protocol), nil)
	}

	keyFileString, ok := keyFile.(string)
	if !ok {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("cannot convert '%v' to string type", keyFile), nil)
	}
	conf := &OPCUAServerConf{
		EndPoint: endpointString,
		Policy:   policyString,
		Mode:     modeString,
		CertFile: certFileString,
		KeyFile:  keyFileString,
	}
	return conf, nil
}

// FetchEndpoint returns the OPCUA endpoint defined in the configuration
func FetchEndpoint(protocols map[string]models.ProtocolProperties) (string, errors.EdgeX) {
	properties, ok := protocols[Protocol]
	if !ok {
		return "", errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' protocol properties is not defined", Protocol), nil)
	}
	endpoint, ok := properties[Endpoint]
	if !ok {
		return "", errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' not found in the '%s' protocol properties", Endpoint, Protocol), nil)
	}
	endpointString, ok := endpoint.(string)
	if !ok {
		return "", errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("cannot convert '%v' to string type", endpointString), nil)
	}
	return endpointString, nil
}
