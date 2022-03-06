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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/internal/oauth"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
)

const callback = "/api/directory/codehosts/callback"

func CreateCodeHost(codehost *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	if codehost.Type == "codehub" {
		codehost.IsReady = "2"
	}
	if codehost.Type == "gerrit" {
		codehost.IsReady = "2"
		codehost.AccessToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", codehost.Username, codehost.Password)))
	}
	codehost.CreatedAt = time.Now().Unix()
	codehost.UpdatedAt = time.Now().Unix()

	list, err := mongodb.NewCodehostColl().CodeHostList()
	if err != nil {
		return nil, err
	}
	codehost.ID = len(list) + 1
	return mongodb.NewCodehostColl().AddCodeHost(codehost)
}

func List(address, owner, source string, _ *zap.SugaredLogger) ([]*models.CodeHost, error) {
	return mongodb.NewCodehostColl().List(&mongodb.ListArgs{
		Address: address,
		Owner:   owner,
		Source:  source,
	})
}

func DeleteCodeHost(id int, _ *zap.SugaredLogger) error {
	return mongodb.NewCodehostColl().DeleteCodeHostByID(id)
}

func UpdateCodeHost(host *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	return mongodb.NewCodehostColl().UpdateCodeHost(host)
}

func UpdateCodeHostByToken(host *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	return mongodb.NewCodehostColl().UpdateCodeHostByToken(host)
}

func GetCodeHost(id int, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	return mongodb.NewCodehostColl().GetCodeHostByID(id)
}

type state struct {
	CodeHostID  int    `json:"code_host_id"`
	RedirectURL string `json:"redirect_url"`
}

func AuthCodeHost(redirectURI string, codeHostID int, logger *zap.SugaredLogger) (string, error) {
	codeHost, err := GetCodeHost(codeHostID, logger)
	if err != nil {
		logger.Errorf("GetCodeHost:%s err:%s", codeHostID, err)
		return "", err
	}
	redirectParsedURL, err := url.Parse(redirectURI)
	if err != nil {
		logger.Errorf("Parse redirectURI:%s err:%s", redirectURI, err)
		return "", err
	}
	redirectParsedURL.Path = callback
	oauth, err := newOAuth(codeHost.Type, redirectParsedURL.String(), codeHost.ApplicationId, codeHost.ClientSecret, codeHost.Address)
	if err != nil {
		logger.Errorf("NewOAuth:%s err:%s", codeHost.Type, err)
		return "", err
	}
	stateStruct := state{
		CodeHostID:  codeHost.ID,
		RedirectURL: redirectURI,
	}
	bs, err := json.Marshal(stateStruct)
	if err != nil {
		logger.Errorf("Marshal err:%s", err)
		return "", err
	}
	return oauth.LoginURL(base64.URLEncoding.EncodeToString(bs)), nil
}

func HandleCallback(stateStr string, r *http.Request, logger *zap.SugaredLogger) (string, error) {
	// TODO：validate the code
	// https://www.jianshu.com/p/c7c8f51713b6
	decryptedState, err := base64.URLEncoding.DecodeString(stateStr)
	if err != nil {
		logger.Errorf("DecodeString err:%s", err)
		return "", err
	}
	var sta state
	if err := json.Unmarshal(decryptedState, &sta); err != nil {
		logger.Errorf("Unmarshal err:%s", err)
		return "", err
	}
	redirectParsedURL, err := url.Parse(sta.RedirectURL)
	if err != nil {
		logger.Errorf("ParseURL:%s err:%s", sta.RedirectURL, err)
		return "", err
	}
	codehost, err := GetCodeHost(sta.CodeHostID, logger)
	if err != nil {
		return handle(redirectParsedURL, err)
	}
	callbackURL := url.URL{
		Scheme: redirectParsedURL.Scheme,
		Host:   redirectParsedURL.Host,
		Path:   callback,
	}
	o, err := newOAuth(codehost.Type, callbackURL.String(), codehost.ApplicationId, codehost.ClientSecret, codehost.Address)
	if err != nil {
		return handle(redirectParsedURL, err)
	}
	token, err := o.HandleCallback(r)
	if err != nil {
		return handle(redirectParsedURL, err)
	}
	codehost.AccessToken = token.AccessToken
	codehost.RefreshToken = token.RefreshToken
	if _, err := UpdateCodeHostByToken(codehost, logger); err != nil {
		logger.Errorf("UpdateCodeHostByToken err:%s", err)
		return handle(redirectParsedURL, err)
	}
	return handle(redirectParsedURL, nil)
}

func newOAuth(provider, callbackURL, clientID, clientSecret, address string) (*oauth.OAuth, error) {
	switch provider {
	case systemconfig.GitHubProvider:
		return oauth.New(callbackURL, clientID, clientSecret, []string{"repo", "user"}, oauth2.Endpoint{
			AuthURL:  address + "/login/oauth/authorize",
			TokenURL: address + "/login/oauth/access_token",
		}), nil
	case systemconfig.GitLabProvider:
		return oauth.New(callbackURL, clientID, clientSecret, []string{"api", "read_user"}, oauth2.Endpoint{
			AuthURL:  address + "/oauth/authorize",
			TokenURL: address + "/oauth/token",
		}), nil
	}
	return nil, errors.New("illegal provider")
}

func handle(url *url.URL, err error) (string, error) {
	if err != nil {
		url.Query().Add("err", err.Error())
	} else {
		url.Query().Add("success", "true")
	}
	return url.String(), nil
}
