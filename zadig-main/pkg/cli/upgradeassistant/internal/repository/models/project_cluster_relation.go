/*
Copyright 2022 The KodeRover Authors.

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
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectClusterRelation struct {
	ID          primitive.ObjectID `json:"id,omitempty"              bson:"_id,omitempty"`
	ProjectName string             `json:"project_name"              bson:"project_name"`
	ClusterID   string             `json:"cluster_id"                bson:"cluster_id"`
	CreatedAt   int64              `json:"createdAt"                 bson:"createdAt"`
	CreatedBy   string             `json:"createdBy"                 bson:"createdBy"`
}

func (ProjectClusterRelation) TableName() string {
	return "project_cluster_relation"
}
