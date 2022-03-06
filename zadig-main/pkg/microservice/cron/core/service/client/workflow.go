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

// ListWorkflows ...
func (c *Client) ListWorkflows(log *zap.SugaredLogger) ([]*service.Workflow, error) {
	var err error
	resp := make([]*service.Workflow, 0)
	var ret *http.Response

	request, err := c.genListWorkFlowReq(log)
	if err != nil {
		return nil, err
	}

	ret, err = c.Conn.Do(request)
	if err != nil {
		return resp, errors.WithMessage(err, "failed to list workflows")
	}

	defer func() {
		_ = ret.Body.Close()
	}()

	var body []byte
	body, err = ioutil.ReadAll(ret.Body)
	if err != nil {
		return resp, errors.WithMessage(err, "failed to list workflows")
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		return resp, errors.WithMessage(err, "failed to list workflows")
	}

	return resp, nil
}

func (c *Client) genListWorkFlowReq(log *zap.SugaredLogger) (*http.Request, error) {
	url := fmt.Sprintf("%s/workflow/workflow", c.APIBase)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("new http request error: %v", err)
		return nil, err
	}

	return request, nil
}
