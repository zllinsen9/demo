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

package cmd

import (
	"fmt"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/shared/client/user"
)

func Healthz() error {
	if err := checkUserServiceHealth(); err != nil {
		return fmt.Errorf("checkUserServiceHealth error:%s", err)
	}
	if err := checkPolicyServiceHealth(); err != nil {
		return fmt.Errorf("checkPolicyServiceHealth error:%s", err)
	}
	if err := checkAslanServiceHealth(); err != nil {
		return fmt.Errorf("checkPolicyServiceHealth error:%s", err)
	}
	return nil
}

func checkUserServiceHealth() error {
	return user.New().Healthz()
}

func checkPolicyServiceHealth() error {
	return policy.NewDefault().Healthz()
}

func checkAslanServiceHealth() error {
	return aslan.New(config.AslanServiceAddress()).Healthz()
}
