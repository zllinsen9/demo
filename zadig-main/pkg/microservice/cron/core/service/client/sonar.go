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
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/pkg/config"
)

func (c *Client) InitPullSonarStatScheduler(log *zap.SugaredLogger) error {
	c.InitPullSonarTestsMeasure(log)
	c.InitPullSonarDeliveryMeasure(log)
	c.InitPullSonarRepos(log)

	return nil
}

func (c *Client) InitPullSonarTestsMeasure(log *zap.SugaredLogger) error {
	log.Info("start to pull sonar test measure..")

	url := fmt.Sprintf("%s/api/quality/sonar/tests/measure/pull", configbase.AslanxServiceAddress())

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Errorf("create post request error : %v", err)
		return err
	}

	var resp *http.Response
	resp, err = c.Conn.Do(request)
	if err != nil {
		log.Errorf("c.Conn.Do error : %v", err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("ioutil.ReadAll error : %v", err)
		return err
	}

	log.Infof("pull sonar test measure result %s", string(result))

	return nil
}

func (c *Client) InitPullSonarDeliveryMeasure(log *zap.SugaredLogger) error {
	log.Info("start to pull sonar delivery measure..")

	url := fmt.Sprintf("%s/api/quality/sonar/delivery/measure/pull", configbase.AslanxServiceAddress())

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Errorf("create post request error : %v", err)
		return err
	}

	var resp *http.Response
	resp, err = c.Conn.Do(request)
	if err != nil {
		log.Errorf("c.Conn.Do error : %v", err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("ioutil.ReadAll error : %v", err)
		return err
	}

	log.Infof("pull sonar delivery measure result %s", string(result))

	return err
}

func (c *Client) InitPullSonarRepos(log *zap.SugaredLogger) error {
	log.Info("start to pull sonar repos..")

	url := fmt.Sprintf("%s/api/quality/sonar/repository/pull", configbase.AslanxServiceAddress())

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Errorf("create post request error : %v", err)
		return err
	}

	var resp *http.Response
	resp, err = c.Conn.Do(request)
	if err != nil {
		log.Errorf("c.Conn.Do error : %v", err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("ioutil.ReadAll error : %v", err)
		return err
	}

	log.Infof("pull sonar repos result %s", string(result))

	return err
}
