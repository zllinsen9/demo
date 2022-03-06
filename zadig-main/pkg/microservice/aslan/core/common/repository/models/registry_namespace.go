/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package models

import (
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
)

type RegistryNamespace struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"               json:"id,omitempty"`
	RegAddr     string             `bson:"reg_addr"                    json:"reg_addr"`
	RegType     string             `bson:"reg_type"                    json:"reg_type"`
	RegProvider string             `bson:"reg_provider"                json:"reg_provider"`
	IsDefault   bool               `bson:"is_default"                  json:"is_default"`
	// Namespace is NOT a required field, this could be empty when the registry is AWS ECR or so.
	// use with CAUTION !!!!
	Namespace  string `bson:"namespace,omitempty"         json:"namespace,omitempty"`
	AccessKey  string `bson:"access_key"                  json:"access_key"`
	SecretKey  string `bson:"secret_key"                  json:"secret_key"`
	Region     string `bson:"region,omitempty"            json:"region,omitempty"`
	UpdateTime int64  `bson:"update_time"                 json:"update_time"`
	UpdateBy   string `bson:"update_by"                   json:"update_by"`
}

func (ns *RegistryNamespace) Validate() error {

	if ns.RegAddr == "" {
		return errors.New("empty reg_addr")
	}

	if ns.RegProvider == "" {
		return errors.New("empty reg_provider")
	}

	// if the registry type is aws ECR, then the RegAddr is the registry's full
	if ns.Namespace == "" && ns.RegProvider != config.RegistryTypeAWS {
		return errors.New("empty namespace")
	}

	return nil
}

func (RegistryNamespace) TableName() string {
	return "registry_namespace"
}

//const CandidateRegType = "candidate"
//
//const DistRegType = "dist"
