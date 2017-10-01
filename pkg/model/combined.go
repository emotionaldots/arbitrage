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

func NormalizeTorrentGroups(v interface{}) ([]GroupAndTorrents, error) {
	gs := make([]GroupAndTorrents, 0)

	switch m := v.(type) {
	case TorrentAndGroup:
		g := GroupAndTorrents{}
		g.Group = m.Group
		g.Torrents = []Torrent{m.Torrent}
		gs = append(gs, g)
	case GroupAndTorrents:
		gs = append(gs, m)
	case GroupWithTorrents:
		gs = append(gs, m.Split())
	case CollageWithGroups:
		for _, gt := range m.TorrentGroups {
			// Sadly collages do not contain all the fields of individual torrents,
			// so we cannot index them at the moment without risking to overwrite
			// previous database fields
			gt.Torrents = nil
			gs = append(gs, gt.Split())
		}
	default:
		return gs, fmt.Errorf("normalize: unsupported type %T", m)
	}

	for _, g := range gs {
		if g.Group.ID == 0 {
			return gs, errors.New("normalize: no group id")
		}
		for i, t := range g.Torrents {
			t.GroupID = int(g.Group.ID)
			g.Torrents[i] = t
			if t.ID == 0 {
				return gs, errors.New("normalize: no torrent id")
			}
		}
	}

	return gs, nil
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
