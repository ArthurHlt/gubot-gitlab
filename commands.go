package gubot_gitlab

import (
	"github.com/urfave/cli"
	"github.com/xanzy/go-gitlab"
	"errors"
	"github.com/ArthurHlt/gubot/robot"
	"fmt"
	"strings"
	"strconv"
)

func (g GitlabApp) Commands(envelop robot.Envelop) []cli.Command {
	return []cli.Command{
		{
			Name:        "assign",
			Usage:       "assign a merge request or an issue to a user",
			Subcommands: []cli.Command{
				{
					Name:  "me",
					Usage: "assign the latest notification for merge request or issue from channel to yourself",
					Action: func(c *cli.Context) error {
						return g.cmdAssignMe(envelop, c)
					},
				},
				{
					Name:  "last",
					Usage: "assign the latest notification for merge request or issue to someone or yourself",
					Action: func(c *cli.Context) error {
						return g.cmdAssignLast(envelop, c)
					},
				},
				{
					Name: "merge-request",
					Aliases: []string{"pull-request", "pr", "mr"},
					Usage: "assign the last merge request received on channel to someone or yourself",
					Action: func(c *cli.Context) error {
						return g.cmdAssignMergeRequest(envelop, c)
					},
				},
				{
					Name: "issue",
					Usage: "assign the last issue received on channel to someone or yourself",
					Action: func(c *cli.Context) error {
						return g.cmdAssignIssue(envelop, c)
					},
				},
			},
		},
		{
			Name:        "merge-request",
			Usage:       "Manipulate merge request",
			Aliases: []string{"pull-request", "pr", "mr"},
			Subcommands: []cli.Command{
				{
					Name:  "list",
					Usage: "List all opened merge request",
					Action: g.cmdMergeRequestList,

				},
				{
					Name:  "see",
					Usage: "See the content of an opened merge request",
					Action: g.cmdMergeRequestSee,
				},
				{
					Name: "assign",
					Usage: "assign a merge request to someone or yourself",
					Action: func(c *cli.Context) error {
						return g.cmdMergeRequestAssign(envelop, c)
					},
				},
			},
		},
		{
			Name:        "issue",
			Usage:       "Manipulate issues",
			Subcommands: []cli.Command{
				{
					Name:  "list",
					Usage: "List all opened issues",
					Action: g.cmdIssueList,
				},
				{
					Name:  "see",
					Usage: "See the content of an opened issue",
					Action: g.cmdIssueSee,

				},
				{
					Name: "assign",
					Usage: "assign an issue to someone or yourself",
					Action: func(c *cli.Context) error {
						return g.cmdIssueAssign(envelop, c)
					},
				},
			},
		},
	}
}
func (g GitlabApp) cmdAssignMe(envelop robot.Envelop, c *cli.Context) error {
	g.cmdAssign(envelop.User.Name, c, &GitlabNotification{
		ChannelName: envelop.ChannelName,
	})
	return nil
}
func (g GitlabApp) cmdAssignLast(envelop robot.Envelop, c *cli.Context) error {
	username := g.usernameFromCommand(envelop, c)
	if username == "" {
		return nil
	}
	g.cmdAssign(username, c, &GitlabNotification{
		ChannelName: envelop.ChannelName,
	})
	return nil
}
func (g GitlabApp) cmdAssignIssue(envelop robot.Envelop, c *cli.Context) error {
	username := g.usernameFromCommand(envelop, c)
	if username == "" {
		return nil
	}
	g.cmdAssign(username, c, &GitlabNotification{
		ChannelName: envelop.ChannelName,
		Type: ISSUE_EVENT_NAME,
	})
	return nil
}

func (g GitlabApp) cmdAssignMergeRequest(envelop robot.Envelop, c *cli.Context) error {
	username := g.usernameFromCommand(envelop, c)
	if username == "" {
		return nil
	}
	g.cmdAssign(username, c, &GitlabNotification{
		ChannelName: envelop.ChannelName,
		Type: MERGE_REQUEST_EVENT_NAME,
	})
	return nil
}
func (g GitlabApp) cmdMergeRequestList(c *cli.Context) error {
	fmt.Fprint(c.App.Writer, g.listNotifs(&GitlabNotification{
		Type: MERGE_REQUEST_EVENT_NAME,
	}, true))
	return nil
}
func (g GitlabApp) cmdIssueList(c *cli.Context) error {
	fmt.Fprint(c.App.Writer, g.listNotifs(&GitlabNotification{
		Type: ISSUE_EVENT_NAME,
	}, true))
	return nil
}
func (g GitlabApp) cmdIssueSee(c *cli.Context) error {
	g.cmdSee(c, ISSUE_EVENT_NAME)
	return nil
}

func (g GitlabApp) cmdIssueAssign(envelop robot.Envelop, c *cli.Context) error {
	g.cmdMergeOrIssueAssign(envelop, c, ISSUE_EVENT_NAME)
	return nil
}
func (g GitlabApp) cmdMergeRequestSee(c *cli.Context) error {
	g.cmdSee(c, MERGE_REQUEST_EVENT_NAME)
	return nil
}
func (g GitlabApp) cmdMergeRequestAssign(envelop robot.Envelop, c *cli.Context) error {
	g.cmdMergeOrIssueAssign(envelop, c, MERGE_REQUEST_EVENT_NAME)
	return nil
}
func (g GitlabApp) cmdMergeOrIssueAssign(envelop robot.Envelop, c *cli.Context, notifType string) {
	username := g.usernameFromCommand(envelop, c)
	if username == "" {
		return
	}
	notifStr := strings.Replace(notifType, "_", " ", -1)
	notifIdStr := c.Args().Get(1)
	if notifIdStr == "" {
		fmt.Fprintln(c.App.Writer, "I need the id retrieve from list to be able to show you the content of the " + notifStr + ".")
		return
	}
	notifId, err := strconv.Atoi(notifIdStr)
	if err != nil {
		fmt.Fprintln(c.App.Writer, "You gave me an incorrect id.")
		return
	}
	where := &GitlabNotification{
		Type: notifType,
	}
	where.ID = uint(notifId)
	g.cmdAssign(username, c, where)

}
func (g GitlabApp) cmdSee(c *cli.Context, notifType string) {
	notifStr := strings.Replace(notifType, "_", " ", -1)
	notifIdStr := c.Args().First()
	if notifIdStr == "" {
		fmt.Fprintln(c.App.Writer, "I need the id retrieve from list to be able to show you the content of the " + notifStr + ".")
		return
	}
	notifId, err := strconv.Atoi(notifIdStr)
	if err != nil {
		fmt.Fprintln(c.App.Writer, "You gave me an incorrect id.")
		return
	}
	where := &GitlabNotification{
		Type: notifType,
	}
	where.ID = uint(notifId)
	var notif GitlabNotification
	robot.Store().Where(&where).First(&notif)
	if notif.ID == 0 {
		fmt.Fprintln(c.App.Writer, "I can't found this " + notifStr + ".")
		return
	}
	fmt.Fprint(c.App.Writer, notif.Message)
}
func (g GitlabApp) usernameFromCommand(envelop robot.Envelop, c *cli.Context) string {
	username := c.Args().First()
	if username == "" {
		fmt.Fprint(c.App.Writer, "I can't assign someone to the last merge request, no user name was given (it can be 'me' or other user name).")
		return ""
	}
	if username == "me" {
		username = envelop.User.Name
	} else {
		username = strings.TrimPrefix(username, "@")
	}
	return username
}
func (g GitlabApp) cmdAssign(username string, c *cli.Context, where *GitlabNotification) {
	var notif GitlabNotification
	robot.Store().Where(where).Order("created_at desc").First(&notif)
	if notif.ID == 0 {
		fmt.Fprint(c.App.Writer, "Sorry there is no issues or merge request opened")
		return
	}
	var err error
	if notif.Type == MERGE_REQUEST_EVENT_NAME {
		err = g.assignMergeRequest(notif, notif.ObjectId, username)
	} else {
		err = g.assignIssue(notif, notif.ObjectId, username)
	}
	typeEvent := strings.Replace(notif.Type, "_", " ", -1)

	if err != nil {
		fmt.Fprintf(c.App.Writer,
			"Sorry I can't assign you to %s with id %d, I had this error: %s",
			typeEvent,
			notif.ID,
			err.Error(),
		)
		return
	}
	fmt.Fprintf(c.App.Writer,
		"@%s have been assigned to the %s with id %d available here: %s",
		username,
		typeEvent,
		notif.ID,
		notif.WebUrl,
	)
	notif.AssignedUser = username
	robot.Store().Save(&notif)
}
func (g GitlabApp) assignIssue(notif GitlabNotification, issueId int, user string) error {
	fUser, err := g.findUser(user)
	if err != nil {
		return err
	}
	var accessLevel gitlab.AccessLevelValue
	member, _, err := g.client.Projects.GetProjectMember(notif.GroupName, fUser.ID)
	if err != nil || member.ID == 0 {
		userGroup, _, err := g.GetGroupMember(notif.GroupName, fUser.ID, nil)
		if err != nil {
			return err
		}
		accessLevel = userGroup.AccessLevel
	} else {
		accessLevel = member.AccessLevel
	}
	if accessLevel < gitlab.MasterPermissions {
		return errors.New("Nice try but you don't have the correct permission.")
	}
	_, _, err = g.client.Issues.UpdateIssue(notif.ProjectID, issueId, &gitlab.UpdateIssueOptions{
		AssigneeID: &fUser.ID,
	})
	if err != nil {
		return err
	}
	return nil
}
func (g GitlabApp) assignMergeRequest(notif GitlabNotification, mrId int, user string) error {
	fUser, err := g.findUser(user)
	if err != nil {
		return err
	}
	var accessLevel gitlab.AccessLevelValue
	member, _, err := g.client.Projects.GetProjectMember(notif.GroupName, fUser.ID)
	if err != nil || member.ID == 0 {
		userGroup, _, err := g.GetGroupMember(notif.GroupName, fUser.ID, nil)
		if err != nil {
			return err
		}
		accessLevel = userGroup.AccessLevel
	} else {
		accessLevel = member.AccessLevel
	}
	if accessLevel < gitlab.MasterPermissions {
		return errors.New("Nice try but you don't have the correct permission.")
	}
	mergeRequest, _, err := g.client.MergeRequests.GetMergeRequest(notif.ProjectID, mrId)
	if err != nil {
		return err
	}
	_, _, err = g.client.MergeRequests.UpdateMergeRequest(notif.ProjectID, mrId, &gitlab.UpdateMergeRequestOptions{
		AssigneeID: &fUser.ID,
		Description: &mergeRequest.Description,
		Title: &mergeRequest.Title,
		TargetBranch: &mergeRequest.TargetBranch,
	})
	if err != nil {
		return err
	}
	return nil
}

func (g GitlabApp) findUser(user string) (*gitlab.User, error) {
	user = g.retrieveChatUser(user)
	fUsers, _, err := g.client.Users.ListUsers(&gitlab.ListUsersOptions{
		Username: &user,
	})
	if err != nil {
		return nil, err
	}
	if len(fUsers) == 0 {
		return nil, errors.New("User " + user + " not found.")
	}
	return fUsers[0], nil
}