package gubot_gitlab

import (
	"github.com/xanzy/go-gitlab"
	"fmt"
	"strconv"
)

func (g GitlabApp) GetGroupMember(gid interface{}, user int, opt *gitlab.UpdateGroupMemberOptions, options ...gitlab.OptionFunc) (*gitlab.GroupMember, *gitlab.Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/members/%d", group, user)

	req, err := g.client.NewRequest("GET", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	grp := new(gitlab.GroupMember)
	resp, err := g.client.Do(req, grp)
	if err != nil {
		return nil, resp, err
	}

	return grp, resp, err
}
func parseID(id interface{}) (string, error) {
	switch v := id.(type) {
	case int:
		return strconv.Itoa(v), nil
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("invalid ID type %#v, the ID must be an int or a string", id)
	}
}
func mapToSliceString(mapString map[string]bool) []string {
	finalSlice := make([]string, 0)
	for key, _ := range mapString {
		finalSlice = append(finalSlice, key)
	}
	return finalSlice
}