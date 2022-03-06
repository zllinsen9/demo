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

package types

type MediumType string

const (
	ObjectMedium MediumType = "object"
	NFSMedium    MediumType = "nfs"
)

type ProvisionType string

const (
	DynamicProvision ProvisionType = "dynamic"
	StaticProvision  ProvisionType = "static"
)

type ObjectProperties struct {
	ID string `json:"id" bson:"id"`
}

type NFSProperties struct {
	ProvisionType    ProvisionType `json:"provision_type"      bson:"provision_type"`
	StorageClass     string        `json:"storage_class"       bson:"storage_class"`
	StorageSizeInGiB int64         `json:"storage_size_in_gib" bson:"storage_size_in_gib"`
	PVC              string        `json:"pvc"                 bson:"pvc"`
}

type Cache struct {
	MediumType       MediumType       `json:"medium_type"       bson:"medium_type"`
	ObjectProperties ObjectProperties `json:"object_properties" bson:"object_properties"`
	NFSProperties    NFSProperties    `json:"nfs_properties"    bson:"nfs_properties"`
}

type CacheDirType string

const (
	WorkspaceCacheDir   CacheDirType = "workspace"
	UserDefinedCacheDir CacheDirType = "user_defined"
)
