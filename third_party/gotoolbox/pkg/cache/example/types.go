package example

import "time"

// +k8s:deepcopy-gen=true
type User struct {
	Name       string    `json:"name"`
	UUID       string    `json:"uuid"`
	Tenant     string    `json:"tenant"`
	Group      []string  `json:"group"`
	Roles      []string  `json:"roles"`
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
}
