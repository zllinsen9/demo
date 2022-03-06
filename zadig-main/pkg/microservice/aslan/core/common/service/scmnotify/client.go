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

package scmnotify

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	gitlabtool "github.com/koderover/zadig/pkg/tool/git/gitlab"
	"github.com/koderover/zadig/pkg/tool/log"
)

type Client struct {
	logger *zap.SugaredLogger
}

func NewClient() *Client {
	return &Client{logger: log.SugaredLogger()}
}

// Comment send comment to gitlab and set comment id in notify
func (c *Client) Comment(notify *models.Notification) error {
	if notify.PrID == 0 {
		return fmt.Errorf("non pr notification not supported yet")
	}

	var err error
	comment := notify.ErrInfo
	if comment == "" {
		if comment, err = notify.CreateCommentBody(); err != nil {
			return fmt.Errorf("failed to create comment body %v", err)
		}
	}

	codeHostDetail, err := systemconfig.New().GetCodeHost(notify.CodehostID)
	if err != nil {
		return errors.Wrapf(err, "codehost %d not found to comment", notify.CodehostID)
	}
	if strings.ToLower(codeHostDetail.Type) == "gitlab" {
		var note *gitlab.Note
		cli, err := gitlabtool.NewClient(codeHostDetail.Address, codeHostDetail.AccessToken, config.ProxyHTTPSAddr(), codeHostDetail.EnableProxy)
		if err != nil {
			c.logger.Errorf("create gitlab client failed err: %v", err)
			return fmt.Errorf("create gitlab client failed err: %v", err)
		}
		if notify.CommentID == "" {
			// create comment
			note, _, err = cli.Notes.CreateMergeRequestNote(
				notify.ProjectID, notify.PrID, &gitlab.CreateMergeRequestNoteOptions{
					Body: &comment,
				},
			)

			if err == nil {
				notify.CommentID = strconv.Itoa(note.ID)
			}
		} else {
			// update comment
			noteID, _ := strconv.Atoi(notify.CommentID)
			_, _, err = cli.Notes.UpdateMergeRequestNote(
				notify.ProjectID, notify.PrID, noteID, &gitlab.UpdateMergeRequestNoteOptions{
					Body: &comment,
				})
		}

		if err != nil {
			return fmt.Errorf("failed to comment gitlab due to %s/%d %v", notify.ProjectID, notify.PrID, err)
		}
	} else if strings.ToLower(codeHostDetail.Type) == gerrit.CodehostTypeGerrit {
		cli := gerrit.NewClient(codeHostDetail.Address, codeHostDetail.AccessToken, config.ProxyHTTPSAddr(), codeHostDetail.EnableProxy)
		for _, task := range notify.Tasks {
			// create task created comment
			if !task.FirstCommented && task.Status == config.TaskStatusReady {
				if e := cli.SetReview(
					notify.ProjectID,
					notify.PrID,
					fmt.Sprintf(""+
						"%s ⏱️ %s/v1/projects/detail/%s/pipelines/multi/%s/%d",
						strings.ToUpper(string(task.Status)),
						notify.BaseURI,
						task.ProductName,
						task.WorkflowName,
						task.ID,
					),
					notify.Label,
					"0",
					notify.Revision,
				); e != nil {
					c.logger.Warnf("failed to set review %v %v %v", task, notify, e)
				}

				task.FirstCommented = true
				continue
			}

			/* set review score*/
			var emoji, score string
			var skip bool
			switch task.Status {
			case config.TaskStatusPass:
				emoji = "✅"
				score = "+1"
			case config.TaskStatusCancelled:
				emoji = "✖️"
				score = "0"
			case config.TaskStatusTimeout, config.TaskStatusFailed:
				emoji = "❌"
				score = "-1"
			default:
				skip = true
			}

			if !skip {
				if e := cli.SetReview(
					notify.ProjectID,
					notify.PrID,
					fmt.Sprintf(""+
						"%s %s %s/v1/projects/detail/%s/pipelines/multi/%s/%d",
						strings.ToUpper(string(task.Status)),
						emoji,
						notify.BaseURI,
						task.ProductName,
						task.WorkflowName,
						task.ID,
					),
					notify.Label,
					score,
					notify.Revision,
				); e != nil {
					c.logger.Warnf("failed to set review %v %v %v", task, notify, e)
				}
			}
		}
	} else {
		return fmt.Errorf("non gitlab source not supported to comment")
	}

	return nil
}
