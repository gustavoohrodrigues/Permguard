package identity

import "os/user"

func UserByID(id string) (name string) {
	u, err := user.LookupId(id)
	if err != nil {
		return id
	}
	return u.Username
}

func GroupByID(id string) (name string) {
	g, err := user.LookupGroupId(id)
	if err != nil {
		return id
	}
	return g.Name
}
