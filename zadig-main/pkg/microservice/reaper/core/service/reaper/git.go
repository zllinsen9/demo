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

package reaper

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	c "github.com/koderover/zadig/pkg/microservice/reaper/core/service/cmd"
	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/meta"
	"github.com/koderover/zadig/pkg/tool/log"
)

func (r *Reaper) RunGitGc(folder string) error {
	envs := r.getUserEnvs()
	cmd := c.Gc()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = envs
	cmd.Dir = path.Join(r.ActiveWorkspace, folder)
	return cmd.Run()
}

func (r *Reaper) runGitCmds() error {

	envs := r.getUserEnvs()
	// 如果存在github代码库，则设置代理，同时保证非github库不走代理
	if r.Ctx.Proxy.EnableRepoProxy && r.Ctx.Proxy.Type == "http" {
		noProxy := ""
		proxyFlag := false
		for _, repo := range r.Ctx.Repos {
			if repo.EnableProxy {
				if !proxyFlag {
					envs = append(envs, fmt.Sprintf("http_proxy=%s", r.Ctx.Proxy.GetProxyURL()))
					envs = append(envs, fmt.Sprintf("https_proxy=%s", r.Ctx.Proxy.GetProxyURL()))
					proxyFlag = true
				}
			} else {
				uri, err := url.Parse(repo.Address)
				if err == nil {
					if noProxy != "" {
						noProxy += ","
					}
					noProxy += uri.Host
				}
			}
		}
		envs = append(envs, fmt.Sprintf("no_proxy=%s", noProxy))
	}

	// 获取git代码
	cmds := make([]*c.Command, 0)

	if r.Ctx.Git == nil {
		r.Ctx.Git = &meta.Git{}
	}

	cmds = append(cmds, &c.Command{Cmd: c.SetConfig("user.email", r.Ctx.Git.GetEmail()), DisableTrace: true})
	cmds = append(cmds, &c.Command{Cmd: c.SetConfig("user.name", r.Ctx.Git.GetUserName()), DisableTrace: true})

	// https://stackoverflow.com/questions/24952683/git-push-error-rpc-failed-result-56-http-code-200-fatal-the-remote-end-hun/36843260
	//cmds = append(cmds, &c.Command{Cmd: c.SetConfig("http.postBuffer", "524288000"), DisableTrace: true})
	cmds = append(cmds, &c.Command{Cmd: c.SetConfig("http.postBuffer", "2097152000"), DisableTrace: true})
	var tokens []string

	for _, repo := range r.Ctx.Repos {
		if repo == nil || len(repo.Name) == 0 {
			continue
		}

		if repo.Source == meta.ProviderGerrit {
			userpass, _ := base64.StdEncoding.DecodeString(repo.OauthToken)
			userpassPair := strings.Split(string(userpass), ":")
			var user, password string
			if len(userpassPair) > 1 {
				password = userpassPair[1]
			}
			user = userpassPair[0]
			repo.User = user
			if password != "" {
				repo.Password = password
				tokens = append(tokens, repo.Password)
			}
		} else if repo.Source == meta.ProviderCodehub {
			tokens = append(tokens, repo.Password)
		}
		tokens = append(tokens, repo.OauthToken)
		cmds = append(cmds, r.buildGitCommands(repo)...)
	}

	for _, c := range cmds {
		cmdOutReader, err := c.Cmd.StdoutPipe()
		if err != nil {
			return err
		}

		outScanner := bufio.NewScanner(cmdOutReader)
		go func() {
			for outScanner.Scan() {
				fmt.Printf("%s\n", r.maskSecret(tokens, outScanner.Text()))
			}
		}()

		cmdErrReader, err := c.Cmd.StderrPipe()
		if err != nil {
			return err
		}

		errScanner := bufio.NewScanner(cmdErrReader)
		go func() {
			for errScanner.Scan() {
				fmt.Printf("%s\n", r.maskSecret(tokens, errScanner.Text()))
			}
		}()

		c.Cmd.Env = envs
		if !c.DisableTrace {
			log.Infof("%s", strings.Join(c.Cmd.Args, " "))
		}
		if err := c.Cmd.Run(); err != nil {
			if c.IgnoreError {
				continue
			}
			return err
		}
	}
	return nil
}

func (r *Reaper) buildGitCommands(repo *meta.Repo) []*c.Command {

	cmds := make([]*c.Command, 0)

	if len(repo.Name) == 0 {
		return cmds
	}

	workDir := filepath.Join(r.ActiveWorkspace, repo.Name)
	if len(repo.CheckoutPath) != 0 {
		workDir = filepath.Join(r.ActiveWorkspace, repo.CheckoutPath)
	}

	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		os.MkdirAll(workDir, 0777)
	}

	// 预防非正常退出导致git被锁住
	indexLockPath := path.Join(workDir, "/.git/index.lock")
	if err := os.RemoveAll(indexLockPath); err != nil {
		log.Errorf("Failed to remove %s: %s", indexLockPath, err)
	}
	shallowLockPath := path.Join(workDir, "/.git/shallow.lock")
	if err := os.RemoveAll(shallowLockPath); err != nil {
		log.Errorf("Failed to remove %s: %s", shallowLockPath, err)
	}

	if isDirEmpty(filepath.Join(workDir, ".git")) {
		cmds = append(cmds, &c.Command{Cmd: c.InitGit(workDir)})
	} else {
		cmds = append(cmds, &c.Command{Cmd: c.RemoteRemove(repo.RemoteName), DisableTrace: true, IgnoreError: true})
	}

	if repo.Source == meta.ProviderGitlab {
		u, _ := url.Parse(repo.Address)
		cmds = append(cmds, &c.Command{
			Cmd:          c.RemoteAdd(repo.RemoteName, r.Ctx.Git.OAuthCloneURL(repo.Source, repo.OauthToken, u.Host, repo.Owner, repo.Name, u.Scheme)),
			DisableTrace: true,
		})
	} else if repo.Source == meta.ProviderGerrit {
		u, _ := url.Parse(repo.Address)
		u.Path = fmt.Sprintf("/a/%s", repo.Name)
		u.User = url.UserPassword(repo.User, repo.Password)

		cmds = append(cmds, &c.Command{
			Cmd:          c.RemoteAdd(repo.RemoteName, u.String()),
			DisableTrace: true,
		})
	} else if repo.Source == meta.ProviderCodehub {
		u, _ := url.Parse(repo.Address)
		user := url.QueryEscape(repo.User)
		cmds = append(cmds, &c.Command{
			Cmd:          c.RemoteAdd(repo.RemoteName, fmt.Sprintf("%s://%s:%s@%s/%s/%s.git", u.Scheme, user, repo.Password, u.Host, repo.Owner, repo.Name)),
			DisableTrace: true,
		})
	} else {
		// github
		cmds = append(cmds, &c.Command{Cmd: c.RemoteAdd(repo.RemoteName, r.Ctx.Git.HTTPSCloneURL(repo.Source, repo.OauthToken, repo.Owner, repo.Name)), DisableTrace: true})
	}

	ref := repo.Ref()
	if ref == "" {
		return cmds
	}

	cmds = append(cmds, &c.Command{Cmd: c.Fetch(repo.RemoteName, ref)}, &c.Command{Cmd: c.CheckoutHead()})

	// PR rebase branch 请求
	if repo.PR > 0 && len(repo.Branch) > 0 {
		newBranch := fmt.Sprintf("pr%d", repo.PR)
		ref := fmt.Sprintf("%s:%s", repo.PRRef(), newBranch)
		cmds = append(
			cmds,
			&c.Command{Cmd: c.DeepenedFetch(repo.RemoteName, repo.BranchRef())},
			&c.Command{Cmd: c.ResetMerge()},
			&c.Command{Cmd: c.DeepenedFetch(repo.RemoteName, ref)},
			&c.Command{Cmd: c.Merge(newBranch)},
		)
	}

	if repo.SubModules {
		cmds = append(cmds, &c.Command{Cmd: c.UpdateSubmodules()})
	}

	cmds = append(cmds, &c.Command{Cmd: c.ShowLastLog()})

	setCmdsWorkDir(workDir, cmds)

	return cmds
}
