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

package s3

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	config2 "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/crypto"
	"github.com/koderover/zadig/pkg/tool/log"
)

type S3 struct {
	*models.S3Storage
}

func (s *S3) GetSchema() string {
	if s.Insecure {
		return "http"
	}
	return "https"
}

func (s *S3) GetEncryptedURL() (encrypted string, err error) {
	return crypto.AesEncrypt(s.GetURL())
}

func (s *S3) GetURL() string {
	return strings.TrimRight(
		fmt.Sprintf(
			"%s://%s:%s@%s/%s/%s", s.GetSchema(), s.Ak, s.Sk, s.Endpoint, s.Bucket, s.Subfolder,
		),
		"/",
	)
}

func NewS3StorageFromURL(uri string) (*S3, error) {
	store, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	sk, _ := store.User.Password()
	paths := strings.Split(strings.TrimLeft(store.Path, "/"), "/")
	bucket := paths[0]

	var subfolder string
	if len(paths) > 1 {
		subfolder = strings.Join(paths[1:], "/")
	}

	ret := &S3{
		&models.S3Storage{
			Ak:        store.User.Username(),
			Sk:        sk,
			Endpoint:  store.Host,
			Bucket:    bucket,
			Subfolder: subfolder,
			Insecure:  store.Scheme == "http",
		},
	}
	if strings.Contains(store.Host, config2.MinioServiceName()) {
		ret.Provider = setting.ProviderSourceSystemDefault
	}

	return ret, nil
}

func NewS3StorageFromEncryptedURI(encryptedURI string) (*S3, error) {
	uri, err := crypto.AesDecrypt(encryptedURI)
	if err != nil {
		return nil, err
	}

	return NewS3StorageFromURL(uri)
}

func (s *S3) GetURI() string {
	return strings.TrimRight(
		fmt.Sprintf(
			"%s://%s/%s/%s", s.GetSchema(), s.Endpoint, s.Bucket, s.Subfolder,
		),
		"/",
	)
}

func (s *S3) GetObjectPath(name string) string {
	// target should not be started with /
	if s.Subfolder != "" {
		return strings.TrimLeft(filepath.Join(s.Subfolder, name), "/")
	}

	return strings.TrimLeft(name, "/")
}

func (s *S3) Validate() error {
	s.Ak = strings.Trim(s.Ak, " ")
	s.Sk = strings.Trim(s.Sk, " ")
	s.Bucket = strings.Trim(s.Bucket, " /")
	s.Subfolder = strings.Trim(s.Subfolder, " /")
	s.Endpoint = strings.Trim(s.Endpoint, " /")

	if s.Ak == "" || s.Sk == "" || s.Bucket == "" || s.Endpoint == "" {
		return errors.New("required field is missing")
	}

	defaultStorage, err := FindDefaultS3()
	if err != nil {
		return fmt.Errorf("failed to find default object storage: %s", err)
	}
	if s.ID == defaultStorage.ID && !s.IsDefault {
		return errors.New("current storage is default and a default object storage must be set")
	}

	return nil
}

func FindDefaultS3() (*S3, error) {
	storage, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		log.Warnf("Failed to find default s3 in db, err: %s", err)
		return &S3{
			S3Storage: &models.S3Storage{
				Ak:       config.S3StorageAK(),
				Sk:       config.S3StorageSK(),
				Endpoint: getEndpoint(),
				Bucket:   config.S3StorageBucket(),
				Insecure: config.S3StorageProtocol() == "http",
				Provider: setting.ProviderSourceSystemDefault,
			},
		}, nil
	}

	return &S3{S3Storage: storage}, nil
}

func getEndpoint() string {
	const svc = "zadig-minio"
	endpoint := config.S3StorageEndpoint()
	newEndpoint := fmt.Sprintf("%s.%s%s", svc, config.Namespace(), strings.TrimPrefix(endpoint, svc))
	return newEndpoint
}

func FindS3ById(id string) (*S3, error) {
	storage, err := commonrepo.NewS3StorageColl().Find(id)
	if err != nil {
		log.Warnf("Failed to find s3 in db, err: %s", err)
		return nil, err
	}

	return &S3{S3Storage: storage}, nil
}

// 获取内置的s3
func FindInternalS3() *S3 {
	storage := &models.S3Storage{
		Ak:       config.S3StorageAK(),
		Sk:       config.S3StorageSK(),
		Endpoint: getEndpoint(),
		Bucket:   config.S3StorageBucket(),
		Insecure: config.S3StorageProtocol() == "http",
	}
	return &S3{S3Storage: storage}
}
