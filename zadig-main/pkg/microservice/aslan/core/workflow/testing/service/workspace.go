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

package service

import (
	"fmt"
	"path"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
)

func GetTestArtifactInfo(pipelineName, dir string, taskID int64, log *zap.SugaredLogger) ([]string, error) {
	fis := make([]string, 0)

	storage, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("GetTestArtifactInfo FindDefaultS3 err:%v", err)
		return fis, err
	}
	if storage.Subfolder != "" {
		storage.Subfolder = fmt.Sprintf("%s/%s/%d/%s", storage.Subfolder, pipelineName, taskID, "artifact")
	} else {
		storage.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, taskID, "artifact")
	}
	forcedPathStyle := true
	if storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("GetTestArtifactInfo create s3 client err:%v", err)
		return fis, err
	}
	prefix := storage.GetObjectPath(dir)
	files, err := client.ListFiles(storage.Bucket, prefix, true)
	if err != nil || len(files) <= 0 {
		log.Errorf("GetTestArtifactInfo ListFiles err:%v", err)
		return fis, err
	}
	resp := make([]string, 0)
	for _, file := range files {
		_, fileName := path.Split(file)
		resp = append(resp, fileName)
	}
	return resp, nil
}
