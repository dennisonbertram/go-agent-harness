package main

type User struct {
	Name string
}

func GetName(u *User) string {
	if u == nil {
		return ""
	}
	return u.Name
}
