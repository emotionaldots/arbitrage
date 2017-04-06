package model

type Collage struct {
	ID                  int                 `json:"id"`
	Name                string              `json:"name"`
	Description         string              `json:"description"`
	CreatorID           int                 `json:"creatorID"`
	Deleted             bool                `json:"deleted"`
	CollageCategoryId   int                 `json:"collageCategoryId"`
	CollageCategoryName string              `json:"collageCategoryName"`
	Locked              bool                `json:"locked"`
	MaxGroups           int                 `json:"maxGroups"`
	MaxGroupsPerUser    int                 `json:"maxGroupsPerUser"`
	HasBookmarked       bool                `json:"hasBookmarked"`
	SubscriberCount     int                 `json:"subscriberCount"`
	TorrentGroupIDList  []int               `json:"torrentGroupIDList"`
	TorrentGroups       []GroupWithTorrents `json:"torrentgroups"`
}
