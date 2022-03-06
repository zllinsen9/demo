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

package github

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/git/github"
)

type Client struct {
	*github.Client
	// InstallationToken string
}

func NewClient(accessToken, proxyAddress string, enableProxy bool) *Client {
	cfg := &github.Config{
		AccessToken: accessToken,
	}
	if enableProxy {
		cfg.Proxy = proxyAddress
	}
	return &Client{
		Client: github.NewClient(cfg),
	}
}

func GetGithubAppClientByOwner(owner string) (*Client, error) {
	githubApps, _ := commonrepo.NewGithubAppColl().Find()
	if len(githubApps) == 0 {
		return nil, nil
	}
	appKey := githubApps[0].AppKey
	appID := githubApps[0].AppID

	gc, err := github.NewAppClient(&github.Config{
		AppKey: appKey,
		AppID:  appID,
		Owner:  owner,
		Proxy:  config.ProxyHTTPSAddr(),
	})

	if err != nil {
		return nil, err
	}

	return &Client{Client: gc}, nil
}
