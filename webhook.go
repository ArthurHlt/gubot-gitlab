package gubot_gitlab

import (
	"io/ioutil"
	"github.com/ArthurHlt/gubot/robot"
	"net/http"
	"encoding/json"
	"github.com/xanzy/go-gitlab"
	"fmt"
	"strings"
)

func (g GitlabApp) incomingWebhook(w http.ResponseWriter, req *http.Request) {
	var gitlabEvent struct {
		ObjectKind string `json:"object_kind"`
	}
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		robot.Logger().Error("Incoming gitlab webhook: %s", err.Error())
		return
	}
	err = json.Unmarshal(b, &gitlabEvent)
	if err != nil {
		robot.Logger().Error("Incoming gitlab webhook: %s", err.Error())
		return
	}
	switch gitlabEvent.ObjectKind {
	case MERGE_REQUEST_EVENT_NAME:
		g.notifyMergeRequest(b)
		break
	case ISSUE_EVENT_NAME:
		g.notifyIssue(b)
		break
	case BUILD_EVENT_NAME:
		g.notifyBuildFailed(b)
		break
	case PIPELINE_EVENT_NAME:
		g.notifyBuildFailed(b)
		break
	default:
		return
	}
}
func (g GitlabApp) notifyPipelineFailed(webhook []byte) {
	var pipelineEvent gitlab.PipelineEvent
	json.Unmarshal(webhook, &pipelineEvent)
	if pipelineEvent.ObjectAttributes.Status != "failed" {
		return
	}
	if g.isFilteredRepo(pipelineEvent.Project.PathWithNamespace) {
		return
	}
	notif := &GitlabNotification{
		ProjectID: pipelineEvent.ObjectAttributes.ID,
		ProjectName: pipelineEvent.Project.PathWithNamespace,
		Type: PIPELINE_EVENT_NAME,
		ObjectId: pipelineEvent.ObjectAttributes.ID,
		ChannelName: g.conf.GitlabNotifyChannel,
		WebUrl: pipelineEvent.Project.WebURL,
	}
	notif.Message = fmt.Sprintf(
		"Pipeline failed on project [%s](%s)",
		notif.ProjectName,
		pipelineEvent.Project.WebURL,
	)

	g.notify(notif)
}
func (g GitlabApp) notifyBuildFailed(webhook []byte) {
	var buildEvent gitlab.BuildEvent
	buildEvent.Repository = &gitlab.Repository{}
	json.Unmarshal(webhook, &buildEvent)
	if buildEvent.BuildStatus != "failed" {
		return
	}
	if g.isFilteredRepo(buildEvent.Repository.PathWithNamespace) {
		return
	}
	notif := &GitlabNotification{
		ProjectID: buildEvent.ProjectID,
		ProjectName: buildEvent.Repository.PathWithNamespace,
		Type: BUILD_EVENT_NAME,
		ObjectId: buildEvent.BuildID,
		ChannelName: g.conf.GitlabNotifyChannel,
		WebUrl: buildEvent.Repository.HTTPURL,
	}
	notif.Message = fmt.Sprintf(
		"Build failed on project [%s](%s)",
		notif.ProjectName,
		buildEvent.Repository.HTTPURL,
	)

	g.notify(notif)
}
func (g GitlabApp) notifyIssue(webhook []byte) {
	var issueEvent gitlab.IssueEvent
	json.Unmarshal(webhook, &issueEvent)
	if g.isFilteredRepo(issueEvent.Project.PathWithNamespace) {
		return
	}
	notif := &GitlabNotification{
		ProjectID: issueEvent.ObjectAttributes.ProjectID,
		ProjectName: issueEvent.Project.Name,
		GroupName: issueEvent.Project.Namespace,
		Type: ISSUE_EVENT_NAME,
		ObjectId: issueEvent.ObjectAttributes.ID,
		ChannelName: g.conf.GitlabNotifyChannel,
		WebUrl: issueEvent.ObjectAttributes.URL,
		ProjectUrl: issueEvent.Project.Homepage,
	}
	if issueEvent.ObjectAttributes.State == "closed" {
		var count int
		robot.Store().Model(&GitlabNotification{}).Where(notif).Count(&count)
		if count > 0 {
			robot.Store().Unscoped().Where(&notif).Delete(GitlabNotification{})
		}
		return
	}
	if issueEvent.ObjectAttributes.State != "opened" {
		return
	}

	notif.Message = fmt.Sprintf(
		"**Issue** on project [%s](%s) from @%s, [click here](%s), title: \n> %s",
		issueEvent.Project.PathWithNamespace,
		issueEvent.ObjectAttributes.URL,
		g.retrieveChatUser(issueEvent.User.Username),
		issueEvent.ObjectAttributes.URL,
		issueEvent.ObjectAttributes.Title,
	)

	g.notifyWithSave(notif)
}
func (g GitlabApp) notifyMergeRequest(webhook []byte) {

	var mergeEvent gitlab.MergeEvent
	json.Unmarshal(webhook, &mergeEvent)
	if g.isFilteredRepo(mergeEvent.Project.PathWithNamespace) {
		return
	}
	notif := &GitlabNotification{
		ProjectID: mergeEvent.ObjectAttributes.TargetProjectID,
		ProjectName: mergeEvent.Project.Name,
		GroupName: mergeEvent.Project.Namespace,
		Type: MERGE_REQUEST_EVENT_NAME,
		ObjectId: mergeEvent.ObjectAttributes.ID,
		ChannelName: g.conf.GitlabNotifyChannel,
		WebUrl: mergeEvent.ObjectAttributes.URL,
		ProjectUrl: mergeEvent.Project.Homepage,
	}
	state := mergeEvent.ObjectAttributes.State
	if state == "closed" || state == "merged" {
		var count int
		robot.Store().Model(&GitlabNotification{}).Where(notif).Count(&count)
		if count > 0 {
			robot.Store().Unscoped().Where(&notif).Delete(GitlabNotification{})
		}
		return
	}
	if mergeEvent.Assignee.Username != "" || mergeEvent.ObjectAttributes.State != "opened" {
		return
	}
	notif.Message = fmt.Sprintf(
		"**Merge request** on project [%s](%s) from @%s, [click here](%s), title: \n> %s",
		mergeEvent.ObjectAttributes.Target.PathWithNamespace,
		mergeEvent.ObjectAttributes.URL,
		g.retrieveChatUser(mergeEvent.User.Username),
		mergeEvent.ObjectAttributes.URL,
		mergeEvent.ObjectAttributes.Title,
	)
	g.notifyWithSave(notif)
}
func (g GitlabApp) notifyWithSave(notif *GitlabNotification) {
	if g.isFilteredRepo(notif.ProjectName) {
		return
	}
	var count int
	robot.Store().Model(&GitlabNotification{}).Where(notif).Count(&count)
	if count > 0 {
		return
	}
	robot.Store().Create(notif)
	g.notify(notif)
}
func (g GitlabApp) notify(notif *GitlabNotification) {
	users, err := g.userWithPermissionFromProjects(*notif)
	if err != nil {
		robot.Logger().Error("Error when notifying: %s", err.Error())
		return
	}
	message := "@" + strings.Join(users, " @") + " : " + notif.Message
	robot.SendMessages(robot.Envelop{
		ChannelName: notif.ChannelName,
	}, message)
}