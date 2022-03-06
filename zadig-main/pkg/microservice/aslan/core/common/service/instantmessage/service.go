/*
Copyright 2022 The KodeRover Authors.

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

package instantmessage

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

const (
	msgType    = "markdown"
	singleInfo = "single"
	multiInfo  = "multi"
)

type BranchTagType string

const (
	BranchTagTypeBranch      BranchTagType = "Branch"
	BranchTagTypeTag         BranchTagType = "Tag"
	CommitMsgInterceptLength               = 60
)

type Service struct {
	proxyColl        *mongodb.ProxyColl
	workflowColl     *mongodb.WorkflowColl
	pipelineColl     *mongodb.PipelineColl
	testingColl      *mongodb.TestingColl
	testTaskStatColl *mongodb.TestTaskStatColl
}

func NewWeChatClient() *Service {
	return &Service{
		proxyColl:        mongodb.NewProxyColl(),
		workflowColl:     mongodb.NewWorkflowColl(),
		pipelineColl:     mongodb.NewPipelineColl(),
		testingColl:      mongodb.NewTestingColl(),
		testTaskStatColl: mongodb.NewTestTaskStatColl(),
	}
}

type wechatNotification struct {
	Task        *task.Task `json:"task"`
	BaseURI     string     `json:"base_uri"`
	IsSingle    bool       `json:"is_single"`
	WebHookType string     `json:"web_hook_type"`
	TotalTime   int64      `json:"total_time"`
	AtMobiles   []string   `json:"atMobiles"`
	IsAtAll     bool       `json:"is_at_all"`
}

func (w *Service) SendMessageRequest(uri string, message interface{}) ([]byte, error) {
	c := httpclient.New()

	// 使用代理
	proxies, _ := w.proxyColl.List(&mongodb.ProxyArgs{})
	if len(proxies) != 0 && proxies[0].EnableApplicationProxy {
		c.SetProxy(proxies[0].GetProxyURL())
		fmt.Printf("send message is using proxy:%s\n", proxies[0].GetProxyURL())
	}

	res, err := c.Post(uri, httpclient.SetBody(message))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (w *Service) SendInstantMessage(task *task.Task, testTaskStatusChanged bool) error {
	var (
		uri         = ""
		content     = ""
		webHookType = ""
		atMobiles   []string
		isAtAll     bool
		title       = ""
		larkCard    *LarkCard
	)
	if task.Type == config.SingleType {
		resp, err := w.pipelineColl.Find(&mongodb.PipelineFindOption{Name: task.PipelineName})
		if err != nil {
			log.Errorf("Pipeline find err :%s", err)
			return err
		}
		if resp.NotifyCtl == nil {
			log.Infof("pipeline notifyCtl is not set!")
			return nil
		}
		if resp.NotifyCtl.Enabled && sets.NewString(resp.NotifyCtl.NotifyTypes...).Has(string(task.Status)) {
			webHookType = resp.NotifyCtl.WebHookType
			if webHookType == dingDingType {
				uri = resp.NotifyCtl.DingDingWebHook
				atMobiles = resp.NotifyCtl.AtMobiles
				isAtAll = resp.NotifyCtl.IsAtAll
			} else if webHookType == feiShuType {
				uri = resp.NotifyCtl.FeiShuWebHook
			} else {
				uri = resp.NotifyCtl.WeChatWebHook
			}
			content, err = w.createNotifyBody(&wechatNotification{
				Task:        task,
				BaseURI:     configbase.SystemAddress(),
				IsSingle:    true,
				WebHookType: webHookType,
				TotalTime:   time.Now().Unix() - task.StartTime,
				AtMobiles:   atMobiles,
				IsAtAll:     isAtAll,
			})
			if err != nil {
				log.Errorf("pipeline createNotifyBody err :%s", err)
				return err
			}
		}
	} else if task.Type == config.WorkflowType {
		resp, err := w.workflowColl.Find(task.PipelineName)
		if err != nil {
			log.Errorf("Workflow find err :%s", err)
			return err
		}
		if resp.NotifyCtl == nil {
			log.Infof("Workflow notifyCtl is not set!")
			return nil
		}
		if resp.NotifyCtl.Enabled && sets.NewString(resp.NotifyCtl.NotifyTypes...).Has(string(task.Status)) {
			webHookType = resp.NotifyCtl.WebHookType
			if webHookType == dingDingType {
				uri = resp.NotifyCtl.DingDingWebHook
				atMobiles = resp.NotifyCtl.AtMobiles
				isAtAll = resp.NotifyCtl.IsAtAll
			} else if webHookType == feiShuType {
				uri = resp.NotifyCtl.FeiShuWebHook
			} else {
				uri = resp.NotifyCtl.WeChatWebHook
			}
			title, content, larkCard, err = w.createNotifyBodyOfWorkflowIM(&wechatNotification{
				Task:        task,
				BaseURI:     configbase.SystemAddress(),
				IsSingle:    false,
				WebHookType: webHookType,
				TotalTime:   time.Now().Unix() - task.StartTime,
				AtMobiles:   atMobiles,
				IsAtAll:     isAtAll,
			})
			if err != nil {
				log.Errorf("workflow createNotifyBodyOfWorkflowIM err :%s", err)
				return err
			}
		}
	} else if task.Type == config.TestType {
		resp, err := w.testingColl.Find(strings.TrimSuffix(task.PipelineName, "-job"), task.ProductName)
		if err != nil {
			log.Errorf("testing find err :%s", err)
			return err
		}
		if resp.NotifyCtl == nil {
			log.Infof("testing notifyCtl is not set!")
			return nil
		}
		statusSets := sets.NewString(resp.NotifyCtl.NotifyTypes...)
		if resp.NotifyCtl.Enabled && (statusSets.Has(string(task.Status)) || (testTaskStatusChanged && statusSets.Has(string(config.StatusChanged)))) {
			webHookType = resp.NotifyCtl.WebHookType
			if webHookType == dingDingType {
				uri = resp.NotifyCtl.DingDingWebHook
				atMobiles = resp.NotifyCtl.AtMobiles
				isAtAll = resp.NotifyCtl.IsAtAll
			} else if webHookType == feiShuType {
				uri = resp.NotifyCtl.FeiShuWebHook
			} else {
				uri = resp.NotifyCtl.WeChatWebHook
			}
			title, content, larkCard, err = w.createNotifyBodyOfTestIM(resp.Desc, &wechatNotification{
				Task:        task,
				BaseURI:     configbase.SystemAddress(),
				IsSingle:    false,
				WebHookType: webHookType,
				TotalTime:   time.Now().Unix() - task.StartTime,
				AtMobiles:   atMobiles,
				IsAtAll:     isAtAll,
			})
			if err != nil {
				log.Errorf("testing createNotifyBodyOfTestIM err :%s", err)
				return err
			}
		}
	}

	if uri != "" && (content != "" || larkCard != nil) {
		if webHookType == dingDingType {
			if task.Type == config.SingleType {
				title = "工作流状态"
			}
			err := w.sendDingDingMessage(uri, title, content, atMobiles)
			if err != nil {
				log.Errorf("sendDingDingMessage err : %s", err)
				return err
			}
		} else if webHookType == feiShuType {
			if task.Type == config.SingleType {
				err := w.sendFeishuMessageOfSingleType("工作流状态", uri, content)
				if err != nil {
					log.Errorf("sendFeishuMessageOfSingleType Request err : %s", err)
					return err
				}
				return nil
			}

			err := w.sendFeishuMessage(uri, larkCard)
			if err != nil {
				log.Errorf("SendFeiShuMessageRequest err : %s", err)
				return err
			}
		} else {
			typeText := weChatTextTypeMarkdown
			if task.Type == config.SingleType {
				typeText = weChatTextTypeText
			}
			err := w.SendWeChatWorkMessage(typeText, uri, content)
			if err != nil {
				log.Errorf("SendWeChatWorkMessage err : %s", err)
				return err
			}
		}
	}
	return nil
}

func (w *Service) createNotifyBody(weChatNotification *wechatNotification) (content string, err error) {
	tmplSource := "{{if eq .WebHookType \"feishu\"}}触发的工作流: {{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/pipelines/{{ isSingle .IsSingle }}/{{.Task.PipelineName}}/{{.Task.TaskID}}{{else}}#### 触发的工作流: [{{.Task.PipelineName}}#{{.Task.TaskID}}]({{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/pipelines/{{ isSingle .IsSingle }}/{{.Task.PipelineName}}/{{.Task.TaskID}}){{end}} \n" +
		"- 状态: {{if eq .WebHookType \"feishu\"}}{{.Task.Status}}{{else}}<font color=\"{{ getColor .Task.Status }}\">{{.Task.Status}}</font>{{end}} \n" +
		"- 创建人：{{.Task.TaskCreator}} \n" +
		"- 总运行时长：{{ .TotalTime}} 秒 \n"

	testNames := getHTMLTestReport(weChatNotification.Task)
	if len(testNames) != 0 {
		tmplSource += "- 测试报告：\n"
	}

	for _, testName := range testNames {
		url := fmt.Sprintf("{{.BaseURI}}/api/aslan/testing/report?pipelineName={{.Task.PipelineName}}&pipelineType={{.Task.Type}}&taskID={{.Task.TaskID}}&testName=%s\n", testName)
		if weChatNotification.WebHookType == feiShuType {
			tmplSource += url
			continue
		}
		tmplSource += fmt.Sprintf("[%s](%s)\n", url, url)
	}

	if weChatNotification.WebHookType == dingDingType {
		if len(weChatNotification.AtMobiles) > 0 && !weChatNotification.IsAtAll {
			tmplSource = fmt.Sprintf("%s - 相关人员：@%s \n", tmplSource, strings.Join(weChatNotification.AtMobiles, "@"))
		}
	}

	tplcontent, err := getTplExec(tmplSource, weChatNotification)
	return tplcontent, err
}

func (w *Service) createNotifyBodyOfWorkflowIM(weChatNotification *wechatNotification) (string, string, *LarkCard, error) {
	tplTitle := "{{if ne .WebHookType \"feishu\"}}#### {{end}}{{getIcon .Task.Status }}{{if eq .WebHookType \"wechat\"}}<font color=\"{{ getColor .Task.Status }}\">工作流{{.Task.PipelineName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}</font>{{else}}工作流 {{.Task.PipelineName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}{{end}} \n"
	tplBaseInfo := []string{"{{if eq .WebHookType \"dingding\"}}##### {{end}}**执行用户**：{{.Task.TaskCreator}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**环境信息**：{{.Task.WorkflowArgs.Namespace}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**开始时间**：{{ getStartTime .Task.StartTime}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**持续时间**：{{ getDuration .TotalTime}} \n",
	}

	build := []string{}
	test := ""
	for _, subStage := range weChatNotification.Task.Stages {
		switch subStage.TaskType {
		case config.TaskBuild:
			for _, sb := range subStage.SubTasks {
				buildElemTemp := ""
				buildSt, err := base.ToBuildTask(sb)
				if err != nil {
					return "", "", nil, err
				}
				branchTag, branchTagType, commitID, commitMsg, gitCommitURL := "", BranchTagTypeBranch, "", "", ""
				for idx, buildRepo := range buildSt.JobCtx.Builds {
					if idx == 0 || buildRepo.IsPrimary {
						branchTag = buildRepo.Branch
						if buildRepo.Tag != "" {
							branchTagType = BranchTagTypeTag
							branchTag = buildRepo.Tag
						}
						if len(buildRepo.CommitID) > 8 {
							commitID = buildRepo.CommitID[0:8]
						}
						commitMsgs := strings.Split(buildRepo.CommitMessage, "\n")
						if len(commitMsgs) > 0 {
							commitMsg = commitMsgs[0]
						}
						if len(commitMsg) > CommitMsgInterceptLength {
							commitMsg = commitMsg[0:CommitMsgInterceptLength]
						}
						gitCommitURL = fmt.Sprintf("%s/%s/%s/commit/%s", buildRepo.Address, buildRepo.RepoOwner, buildRepo.RepoName, commitID)
					}
				}
				if buildSt.BuildStatus.Status == "" {
					buildSt.BuildStatus.Status = config.StatusNotRun
				}
				buildElemTemp += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**服务名称**：%s \n", buildSt.ServiceName)
				buildElemTemp += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**镜像信息**：%s \n", buildSt.JobCtx.Image)
				buildElemTemp += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**代码信息**：[%s-%s %s](%s) \n", branchTagType, branchTag, commitID, gitCommitURL)
				buildElemTemp += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**提交信息**：%s \n", commitMsg)
				build = append(build, buildElemTemp)
			}

		case config.TaskTestingV2:
			test = "{{if eq .WebHookType \"dingding\"}}##### {{end}}**测试结果** \n"
			for _, sb := range subStage.SubTasks {
				test = genTestCaseText(test, sb, weChatNotification.Task.TestReports)
			}
		}
	}

	buttonContent := "点击查看更多信息"
	workflowDetailURL := "{{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/pipelines/{{ isSingle .IsSingle }}/{{.Task.PipelineName}}/{{.Task.TaskID}}"
	moreInformation := fmt.Sprintf("[%s](%s)", buttonContent, workflowDetailURL)
	tplTitle, _ = getTplExec(tplTitle, weChatNotification)

	if weChatNotification.WebHookType != feiShuType {
		tplcontent := strings.Join(tplBaseInfo, "")
		tplcontent += strings.Join(build, "")
		tplcontent = fmt.Sprintf("%s%s", tplcontent, test)
		if weChatNotification.WebHookType == dingDingType {
			if len(weChatNotification.AtMobiles) > 0 && !weChatNotification.IsAtAll {
				tplcontent = fmt.Sprintf("%s{{if eq .WebHookType \"dingding\"}}##### {{end}}**相关人员**：@%s \n", tplcontent, strings.Join(weChatNotification.AtMobiles, "@"))
			}
		}
		tplcontent = fmt.Sprintf("%s%s%s", tplTitle, tplcontent, moreInformation)
		tplExecContent, _ := getTplExec(tplcontent, weChatNotification)
		return tplTitle, tplExecContent, nil, nil
	}

	lc := NewLarkCard()
	lc.SetConfig(true)
	lc.SetHeader(getColorTemplateWithStatus(weChatNotification.Task.Status), tplTitle, feiShuTagText)
	for idx, feildContent := range tplBaseInfo {
		feildExecContent, _ := getTplExec(feildContent, weChatNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, idx == 0)
	}
	for _, feildContent := range build {
		feildExecContent, _ := getTplExec(feildContent, weChatNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, true)
	}
	if test != "" {
		test, _ = getTplExec(test, weChatNotification)
		lc.AddI18NElementsZhcnFeild(test, true)
	}
	workflowDetailURL, _ = getTplExec(workflowDetailURL, weChatNotification)
	lc.AddI18NElementsZhcnAction(buttonContent, workflowDetailURL)
	return "", "", lc, nil
}

func (w *Service) createNotifyBodyOfTestIM(desc string, weChatNotification *wechatNotification) (string, string, *LarkCard, error) {

	tplTitle := "{{if ne .WebHookType \"feishu\"}}#### {{end}}{{getIcon .Task.Status }}{{if eq .WebHookType \"wechat\"}}<font color=\"{{ getColor .Task.Status }}\">工作流{{.Task.PipelineName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}</font>{{else}}工作流 {{.Task.PipelineName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}{{end}} \n"
	tplBaseInfo := []string{"{{if eq .WebHookType \"dingding\"}}##### {{end}}**执行用户**：{{.Task.TaskCreator}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**持续时间**：{{ getDuration .TotalTime}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**开始时间**：{{ getStartTime .Task.StartTime}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**测试描述**：" + desc + " \n",
	}

	tplTestCaseInfo := "{{if eq .WebHookType \"dingding\"}}##### {{end}}**测试结果** \n"
	for _, stage := range weChatNotification.Task.Stages {
		if stage.TaskType != config.TaskTestingV2 {
			continue
		}
		for _, subTask := range stage.SubTasks {
			tplTestCaseInfo = genTestCaseText(tplTestCaseInfo, subTask, weChatNotification.Task.TestReports)
		}
	}

	buttonContent := "点击查看更多信息"
	workflowDetailURL := "{{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/test/detail/function/{{.Task.PipelineName}}/{{.Task.TaskID}}"
	moreInformation := fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}[%s](%s)", buttonContent, workflowDetailURL)

	tplTitle, _ = getTplExec(tplTitle, weChatNotification)

	if weChatNotification.WebHookType != feiShuType {
		tplcontent := strings.Join(tplBaseInfo, "")
		tplcontent = fmt.Sprintf("%s%s", tplcontent, tplTestCaseInfo)
		if weChatNotification.WebHookType == dingDingType {
			if len(weChatNotification.AtMobiles) > 0 && !weChatNotification.IsAtAll {
				tplcontent = fmt.Sprintf("%s{{if eq .WebHookType \"dingding\"}}##### {{end}}**相关人员**：@%s \n", tplcontent, strings.Join(weChatNotification.AtMobiles, "@"))
			}
		}
		tplcontent = fmt.Sprintf("%s%s%s", tplTitle, tplcontent, moreInformation)
		tplExecContent, _ := getTplExec(tplcontent, weChatNotification)
		return tplTitle, tplExecContent, nil, nil
	}
	lc := NewLarkCard()
	lc.SetConfig(true)
	lc.SetHeader(getColorTemplateWithStatus(weChatNotification.Task.Status), tplTitle, feiShuTagText)
	for idx, feildContent := range tplBaseInfo {
		feildExecContent, _ := getTplExec(feildContent, weChatNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, idx == 0)
	}
	if tplTestCaseInfo != "" {
		tplTestCaseInfo, _ = getTplExec(tplTestCaseInfo, weChatNotification)
		lc.AddI18NElementsZhcnFeild(tplTestCaseInfo, true)
	}
	workflowDetailURL, _ = getTplExec(workflowDetailURL, weChatNotification)
	lc.AddI18NElementsZhcnAction(buttonContent, workflowDetailURL)

	return "", "", lc, nil
}

func getHTMLTestReport(task *task.Task) []string {
	if task.Type != config.WorkflowType {
		return nil
	}

	testNames := make([]string, 0)
	for _, stage := range task.Stages {
		if stage.TaskType != config.TaskTestingV2 {
			continue
		}

		for testName, subTask := range stage.SubTasks {
			testInfo, err := base.ToTestingTask(subTask)
			if err != nil {
				log.Errorf("parse testInfo failed, err:%s", err)
				continue
			}

			if testInfo.JobCtx.TestType == setting.FunctionTest && testInfo.JobCtx.TestReportPath != "" {
				testNames = append(testNames, testName)
			}
		}
	}

	return testNames
}

func getTplExec(tplcontent string, weChatNotification *wechatNotification) (string, error) {
	tmpl := template.Must(template.New("notify").Funcs(template.FuncMap{
		"getColor": func(status config.Status) string {
			if status == config.StatusPassed {
				return markdownColorInfo
			} else if status == config.StatusTimeout || status == config.StatusCancelled {
				return markdownColorComment
			} else if status == config.StatusFailed {
				return markdownColorWarning
			}
			return markdownColorComment
		},
		"isSingle": func(isSingle bool) string {
			if isSingle {
				return singleInfo
			}
			return multiInfo
		},
		"taskStatus": func(status config.Status) string {
			if status == config.StatusPassed {
				return "执行成功"
			} else if status == config.StatusCancelled {
				return "执行取消"
			} else if status == config.StatusTimeout {
				return "执行超时"
			}
			return "执行失败"
		},
		"getIcon": func(status config.Status) string {
			if status == config.StatusPassed {
				return "👍"
			}
			return "⚠️"
		},
		"getStartTime": func(startTime int64) string {
			return time.Unix(startTime, 0).Format("2006-01-02 15:04:05")
		},
		"getDuration": func(startTime int64) string {
			duration, er := time.ParseDuration(strconv.FormatInt(startTime, 10) + "s")
			if er != nil {
				log.Errorf("getTplExec ParseDuration err:%s", er)
				return "0s"
			}
			return duration.String()
		},
	}).Parse(tplcontent))

	buffer := bytes.NewBufferString("")
	if err := tmpl.Execute(buffer, &weChatNotification); err != nil {
		log.Errorf("getTplExec Execute err:%s", err)
		return "", fmt.Errorf("getTplExec Execute err:%s", err)

	}
	return buffer.String(), nil
}

func checkTestReportsExist(testModuleName string, testReports map[string]interface{}) bool {
	for testname := range testReports {
		if testname == testModuleName {
			return true
		}
	}
	return false
}

func genTestCaseText(test string, subTask, testReports map[string]interface{}) string {
	testSt, err := base.ToTestingTask(subTask)
	if err != nil {
		log.Errorf("parse testInfo failed, err:%s", err)
		return test
	}
	if testSt.TaskStatus == "" {
		testSt.TaskStatus = config.StatusNotRun
	}
	if testSt.JobCtx.TestType == setting.FunctionTest && testSt.JobCtx.TestReportPath != "" && testSt.TaskStatus == config.StatusPassed {
		url := fmt.Sprintf("{{.BaseURI}}/api/aslan/testing/report?pipelineName={{.Task.PipelineName}}&pipelineType={{.Task.Type}}&taskID={{.Task.TaskID}}&testName=%s", testSt.TestModuleName)
		test += fmt.Sprintf("{{if ne .WebHookType \"feishu\"}} - {{end}}[%s](%s): ", testSt.TestModuleName, url)
	} else {
		test += fmt.Sprintf("{{if ne .WebHookType \"feishu\"}} - {{end}}%s: ", testSt.TestModuleName)
	}
	if testReports == nil || !checkTestReportsExist(testSt.TestModuleName, testReports) {
		test += fmt.Sprintf("%s \n", testSt.TaskStatus)
		return test
	}

	for testname, report := range testReports {
		if testname != testSt.TestModuleName {
			continue
		}
		tr := &task.TestReport{}
		if task.IToi(report, tr) != nil {
			log.Errorf("parse TestReport failed, err:%s", err)
			continue
		}
		if tr.FunctionTestSuite == nil {
			test += fmt.Sprintf("%s \n", testSt.TaskStatus)
			continue
		}
		totalNum := tr.FunctionTestSuite.Tests + tr.FunctionTestSuite.Skips
		failedNum := tr.FunctionTestSuite.Failures + tr.FunctionTestSuite.Errors
		successNum := tr.FunctionTestSuite.Tests - failedNum
		test += fmt.Sprintf("%d(成功)%d(失败)%d(总数) \n", successNum, failedNum, totalNum)
	}
	return test
}
