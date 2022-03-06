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

package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/cron/core/service"
)

func (c *Client) GetWebHookUser(log *zap.SugaredLogger) (*service.DomainWebHookUser, error) {
	var (
		domainWebHookUser *service.DomainWebHookUser
		err               error
	)

	url := fmt.Sprintf("%s/stat/webHook/domain", c.APIBase)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("GetWebHookUser new http request error: %v", err)
		return domainWebHookUser, err
	}

	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		var body []byte
		body, err = ioutil.ReadAll(ret.Body)
		if err == nil {
			if err = json.Unmarshal(body, &domainWebHookUser); err == nil {
				return domainWebHookUser, nil
			}
		}
	}

	return domainWebHookUser, errors.WithMessage(err, "failed to get webHook user count")
}
