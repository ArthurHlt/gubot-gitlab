package gubot_gitlab

import (
	"bytes"
	"github.com/urfave/cli"
	"github.com/ArthurHlt/gubot/robot"
	"github.com/xanzy/go-gitlab"
	"strings"
	"github.com/olebedev/emitter"
	"errors"
	"time"
	"net/http"
	"fmt"
	"os"
)

const (
	ROUTE_WEBHOOK = "/gitlab/webhook"
	CRON_CREATE_HOOK_TICK int = 10
	MERGE_REQUEST_EVENT_NAME = "merge_request"
	ISSUE_EVENT_NAME = "issue"
	BUILD_EVENT_NAME = "build"
	PIPELINE_EVENT_NAME = "pipeline"
)

func init() {
	var conf GitlabConfig
	robot.GetConfig(&conf)
	if conf.GitlabToken == "" {
		robot.Logger().Error("GitlabToken conf parameter must be set.")
		os.Exit(1)
	}
	if conf.GitlabBaseUrl == "" {
		robot.Logger().Error("GitlabToken conf parameter must be set.")
		os.Exit(1)
	}
	if conf.GitlabNotifyChannel == "" {
		robot.Logger().Error("GitlabNotifyChannel conf parameter must be set.")
		os.Exit(1)
	}
	robot.On(robot.EVENT_ROBOT_INITIALIZED_STORE, func(emitter *emitter.Event) {
		robot.Store().AutoMigrate(&GitlabHook{})
		robot.Store().AutoMigrate(&GitlabNotification{})
	})
	client := gitlab.NewClient(robot.HttpClient(), conf.GitlabToken)
	client.SetBaseURL(conf.GitlabBaseUrl)
	gitlabApp := NewGitlabApp(client, conf)
	gitlabApp.Listen()

	robot.Router().HandleFunc(ROUTE_WEBHOOK, gitlabApp.incomingWebhook)
	robot.On(robot.EVENT_ROBOT_STARTED, func(emitter *emitter.Event) {
		fmt.Println(gitlabApp.conf)
		gitlabApp.cronHooks()
		gitlabApp.cronNotifications()
	})

	confMatcher := make([]string, 0)
	commands := gitlabApp.Commands(robot.Envelop{})
	for _, command := range commands {
		confMatcher = append(confMatcher, command.Name)
		confMatcher = append(confMatcher, command.Aliases...)
	}
	robot.RegisterScript(robot.Script{
		Name: "Gitlab",
		Description: "Use gitlab command directly in your prefered chat system.",

		Matcher: "(?i)^gitlab ((help|" + strings.Join(confMatcher, "|") + ").*)",
		Function: func(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
			app := cli.NewApp()
			buf := new(bytes.Buffer)
			app.Writer = buf
			app.HideVersion = true
			app.Name = "gitlab"
			app.UsageText = ""
			app.Usage = "use gitlab commands directly in your favorite chat system."
			app.Commands = gitlabApp.Commands(envelop)
			args := make([]string, 0)
			for _, arg := range strings.Split(envelop.Message, " ") {
				arg = strings.TrimSpace(arg)
				if arg == "" {
					continue
				}
				args = append(args, arg)
			}
			app.Run(args)
			return []string{buf.String()}, nil
		},
		Type: robot.Tsend,
	})
}

type GitlabConfig struct {
	GitlabToken          string
	GitlabNotifyInMinute int `cloud:",default=30"`
	GitlabNotifyChannel  string
	GitlabBaseUrl        string
	GitlabFilteredRepos  []string
	GitlabUsersMap       map[string]string
}
type GitlabApp struct {
	client *gitlab.Client
	conf   GitlabConfig
}

func NewGitlabApp(client *gitlab.Client, conf GitlabConfig) *GitlabApp {
	return &GitlabApp{
		client: client,
		conf: conf,
	}
}
func (g GitlabApp) createHooks() error {
	projects, _, err := g.client.Projects.ListProjects(&gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 500,
		},
	})
	if err != nil {
		return err
	}
	listErr := make([]string, 0)
	for _, project := range projects {
		if g.isFilteredRepo(project.PathWithNamespace) {
			continue
		}
		var count int
		robot.Store().Model(&GitlabHook{}).Where(&GitlabHook{
			ProjectID: project.ID,
		}).Count(&count)
		if count > 0 {
			continue
		}

		url := robot.Host() + ROUTE_WEBHOOK
		trueBool := true
		skipInsecure := !robot.HttpClient().Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify
		_, resp, err := g.client.Projects.AddProjectHook(project.ID, &gitlab.AddProjectHookOptions{
			URL: &url,
			MergeRequestsEvents: &trueBool,
			IssuesEvents: &trueBool,
			BuildEvents: &trueBool,
			PipelineEvents: &trueBool,
			EnableSSLVerification: &skipInsecure,
		})
		if resp != nil && resp.StatusCode == 403 {
			robot.Logger().Info("Skipping project %s because you don't have correct permission to create hook", project.Name)
			robot.Store().Create(&GitlabHook{
				ProjectID: project.ID,
				ProjectName: project.NameWithNamespace,
				Skipped: true,
			})
			continue
		}
		if err != nil {
			listErr = append(listErr, err.Error())
			continue
		}
		robot.Store().Create(&GitlabHook{
			ProjectID: project.ID,
			ProjectName: project.NameWithNamespace,
		})
	}
	if len(listErr) > 0 {
		return errors.New(strings.Join(listErr, "\n"))
	}
	return nil
}
func (g GitlabApp) isFilteredRepo(repo string) bool {
	for _, repoFiltered := range g.conf.GitlabFilteredRepos {
		if repoFiltered == repo {
			return true
		}
	}
	return false
}
func (g GitlabApp) cronNotifications() {
	go func() {
		for {
			g.notifAll()
			time.Sleep(time.Duration(CRON_CREATE_HOOK_TICK) * time.Minute)
		}
	}()
}
func (g GitlabApp) notifAll() {
	var notifs []GitlabNotification
	robot.Store().Find(&notifs)
	for _, notif := range notifs {
		notifTime := notif.UpdatedAt.Add(time.Duration(g.conf.GitlabNotifyInMinute) * time.Minute)
		if notif.AssignedUser != "" || notifTime.After(time.Now()) {
			continue
		}
		robot.Store().Save(&notif)
		notif.Message = "Guys, don't forget -- " + notif.Message
		g.notify(&notif)
	}
}
func (g GitlabApp) cronHooks() {
	go func() {
		for {
			err := g.createHooks()
			if err != nil {
				robot.Logger().Error("Error when creating required webhooks: %s", err.Error())
			}
			time.Sleep(time.Duration(CRON_CREATE_HOOK_TICK) * time.Minute)
		}
	}()
}
func (g GitlabApp) userWithPermissionFromProjects(notif GitlabNotification) ([]string, error) {
	var members []*gitlab.ProjectMember
	var err error
	if notif.ProjectID != 0 {
		members, _, err = g.client.Projects.ListProjectMembers(notif.ProjectID, nil)
	} else {
		members, _, err = g.client.Projects.ListProjectMembers(notif.GroupName + "/" + notif.ProjectName, nil)
	}

	if err != nil {
		return []string{}, err
	}

	users := make(map[string]bool)
	for _, member := range members {
		if member.AccessLevel < gitlab.MasterPermissions {
			continue
		}
		users[g.retrieveChatUser(member.Username)] = true
	}
	groupMembers, _, err := g.client.Groups.ListGroupMembers(notif.GroupName, nil)
	if err != nil {
		return mapToSliceString(users), nil
	}
	for _, member := range groupMembers {
		if member.AccessLevel < gitlab.MasterPermissions {
			continue
		}
		users[g.retrieveChatUser(member.Username)] = true
	}
	return mapToSliceString(users), nil
}
func (g GitlabApp) retrieveChatUser(username string) string {
	if username == "" {
		return username
	}
	if _, ok := g.conf.GitlabUsersMap[username]; ok {
		return g.conf.GitlabUsersMap[username]
	}
	return username
}
func (g GitlabApp) retrieveGitlabUser(username string) string {
	if username == "" {
		return username
	}
	for usernameGitlab, _ := range g.conf.GitlabUsersMap {
		if usernameGitlab == username {
			return usernameGitlab
		}
	}
	return username
}
func (g GitlabApp) listNotifs(where *GitlabNotification, showAssigned bool) string {
	message := ""
	var notifs []GitlabNotification
	if where != nil {
		robot.Store().Where(where).Order("project_id asc").Find(&notifs)
	} else {
		robot.Store().Order("project_id asc").Find(&notifs)
	}
	currentProject := 0
	if len(notifs) == 0 {
		typeNotifTxt := "issues or merge requests"
		if where.Type != "" {
			typeNotifTxt = strings.Replace(where.Type, "_", " ", -1)
		}
		return "There is no " + typeNotifTxt + " in queue."
	}
	for _, notif := range notifs {
		if g.isFilteredRepo(notif.GroupName + "/" + notif.ProjectName) {
			continue
		}
		if currentProject != notif.ProjectID {
			message += fmt.Sprintf("- Project [%s](%s)\n", notif.ProjectName, notif.ProjectUrl)
			currentProject = notif.ProjectID
		}
		typeNotif := strings.Replace(notif.Type, "_", " ", -1)
		message += fmt.Sprintf("  - %s: [#%d](%s) ", typeNotif, notif.ID, notif.WebUrl)
		if showAssigned && notif.AssignedUser == "" {
			message += fmt.Sprintf(
				" -- this %s is not assigned, assign to you by doing `gitlab %s assign me %d`",
				typeNotif,
				strings.Replace(notif.Type, "_", "-", -1),
				notif.ID,
			)
		}
		if showAssigned && notif.AssignedUser != "" {
			message += fmt.Sprintf(
				" -- Assigned to %s",
				notif.AssignedUser,
			)
		}
		message += "\n"
	}
	return message
}
func (g GitlabApp) Listen() {
	go func() {
		for event := range robot.On(robot.EVENT_ROBOT_USER_ONLINE) {
			gubotEvent := robot.ToGubotEvent(event)
			username := gubotEvent.Envelop.User.Name
			if username == "" {
				continue
			}
			var notif GitlabNotification
			robot.Store().Where(&GitlabNotification{
				AssignedUser: gubotEvent.Envelop.User.Name,
			}).Order("updated_at desc").First(&notif)
			if notif.ID == 0 {
				continue
			}
			notifTime := notif.UpdatedAt.Add(time.Duration(24) * time.Hour)
			if notifTime.After(time.Now()) {
				continue
			}

			robot.SendMessages(gubotEvent.Envelop,
				"Dont forget that you have merge requests and issues in queues:\n" +
					g.listNotifs(
						&GitlabNotification{
							AssignedUser: gubotEvent.Envelop.User.Name,
						}, false),
			)
		}
	}()
}

