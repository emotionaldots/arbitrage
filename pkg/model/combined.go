package model

import (
	"errors"
	"fmt"
)

type TorrentAndGroup struct {
	Torrent Torrent `json:"torrent"`
	Group   Group   `json:"group"`
}

type GroupAndTorrents struct {
	Group    Group     `json:"group"`
	Torrents []Torrent `json:"torrents"`
}

func (v GroupAndTorrents) Join() GroupWithTorrents {
	return GroupWithTorrents{v.Group, v.Torrents}
}

type GroupWithTorrents struct {
	Group
	Torrents []Torrent `json:"torrents"`
}

func (v GroupWithTorrents) Split() GroupAndTorrents {
	return GroupAndTorrents{v.Group, v.Torrents}
}

func NormalizeTorrentGroups(v interface{}) (GroupAndTorrents, error) {
	g := GroupAndTorrents{}

	switch m := v.(type) {
	case TorrentAndGroup:
		g.Group = m.Group
		g.Torrents = []Torrent{m.Torrent}
	case GroupAndTorrents:
		g = m
	case GroupWithTorrents:
		g = m.Split()
	default:
		return g, fmt.Errorf("normalize: unsupported type %T", m)
	}

	if g.Group.ID == 0 {
		return g, errors.New("normalize: no group id")
	}
	for i, t := range g.Torrents {
		t.GroupID = g.Group.ID
		g.Torrents[i] = t
		if t.ID == 0 {
			return g, errors.New("normalize: no torrent id")
		}
	}

	return g, nil
}

func (gt GroupAndTorrents) String() string {
	str := ""
	numArtists := len(gt.Group.MusicInfo.Artists)
	if numArtists == 1 {
		str += gt.Group.MusicInfo.Artists[0].Name + " - "
	} else if numArtists > 1 {
		str += "Various Artists - "
	}
	str += gt.Group.Name + " ["
	for i, t := range gt.Torrents {
		if i > 0 {
			str += ", "
		}
		str += t.Media + "-" + t.Format
	}
	return str + "]"
}
