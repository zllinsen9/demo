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

package orm

import (
	"gorm.io/gorm"

	"github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
)

// CreateUserLogin add a userLogin record
func CreateUserLogin(userLogin *models.UserLogin, db *gorm.DB) error {
	if err := db.Create(&userLogin).Error; err != nil {
		return err
	}
	return nil
}

// GetUserLogin Get a userLogin based on uid
func GetUserLogin(uid string, account string, loginType config.LoginType, db *gorm.DB) (*models.UserLogin, error) {
	var userLogin models.UserLogin
	err := db.Where("uid = ? and login_id = ? and login_type = ?", uid, account, loginType).First(&userLogin).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &userLogin, nil
}

// DeleteUserLoginByUids Delete  userLogin based on uids
func DeleteUserLoginByUids(uids []string, db *gorm.DB) error {
	var userLogin models.UserLogin
	err := db.Where("uid in ?", uids).Delete(&userLogin).Error
	if err != nil {
		return err
	}
	return nil
}

// DeleteUserLoginByUid Delete  userLogin based on uids
func DeleteUserLoginByUid(uid string, db *gorm.DB) error {
	var userLogin models.UserLogin
	err := db.Where("uid = ?", uid).Delete(&userLogin).Error
	if err != nil {
		return err
	}
	return nil
}

// ListUserLogins Get a userLogin based on uid list
func ListUserLogins(uids []string, db *gorm.DB) (*[]models.UserLogin, error) {
	var userLogins []models.UserLogin
	err := db.Find(&userLogins, "uid in ?", uids).Error
	if err != nil {
		return nil, err
	}
	return &userLogins, nil
}

// UpdateUserLogin update login info
func UpdateUserLogin(uid string, userLogin *models.UserLogin, db *gorm.DB) error {
	if err := db.Model(&models.UserLogin{}).Where("uid = ?", uid).Updates(userLogin).Error; err != nil {
		return err
	}
	return nil
}
