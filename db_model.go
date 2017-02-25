package gubot_gitlab

import "github.com/jinzhu/gorm"

type GitlabHook struct {
	gorm.Model
	ProjectID   int
	ProjectName string
	Skipped     bool
}

type GitlabNotification struct {
	gorm.Model
	ProjectID    int
	ProjectName  string
	GroupName    string
	Type         string
	ObjectId     int
	Message      string
	ChannelName  string
	WebUrl       string
	ProjectUrl   string
	AssignedUser string
}
