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

package config

import (
	"strings"

	"github.com/spf13/viper"

	// init the config first
	_ "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
)

func WarpDrivePodName() string {
	return viper.GetString(setting.WarpDrivePodName)
}

func NSQLookupAddrs() []string {
	return strings.Split(viper.GetString(setting.ENVNsqLookupAddrs), ",")
}

func ReleaseImageTimeout() string {
	return viper.GetString(setting.ReleaseImageTimeout)
}

func Home() string {
	return viper.GetString(setting.Home)
}

func DefaultRegistryAddr() string {
	return viper.GetString(setting.DefaultRegistryAddr)
}

func DefaultRegistryAK() string {
	return viper.GetString(setting.DefaultRegistryAK)
}

func DefaultRegistrySK() string {
	return viper.GetString(setting.DefaultRegistrySK)
}
