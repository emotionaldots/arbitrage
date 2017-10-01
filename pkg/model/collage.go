package model

import "fmt"

type Collage struct {
	ID                  int      `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	CreatorID           int      `json:"creatorID"`
	Deleted             bool     `json:"deleted"`
	CollageCategoryId   int      `json:"collageCategoryId"`
	CollageCategoryName string   `json:"collageCategoryName"`
	Locked              bool     `json:"locked"`
	MaxGroups           int      `json:"maxGroups"`
	MaxGroupsPerUser    int      `json:"maxGroupsPerUser"`
	HasBookmarked       bool     `json:"hasBookmarked"`
	SubscriberCount     int      `json:"subscriberCount"`
	TorrentGroupIDList  []string `json:"torrentGroupIDList,string" sql:"-"`
}

type CollageWithGroups struct {
	Collage
	TorrentGroups []GroupWithTorrents `json:"torrentgroups" sql:"-"`
}

func (c Collage) String() string {
	return fmt.Sprintf("collage %d: %s", c.ID, c.Name)
}

type CollagesTorrents struct {
	ID        int
	CollageID int
	GroupID   int
}
