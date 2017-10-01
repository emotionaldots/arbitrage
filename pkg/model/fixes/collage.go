package fixes

import "github.com/emotionaldots/arbitrage/pkg/model"

type CollageWithStringedGroups struct {
	model.Collage
	TorrentGroups []StringedGroupWithTorrents `json:"torrentgroups" sql:"-"`
}

type StringedGroupWithTorrents struct {
	StringedGroup
	Torrents []model.Torrent `json:"torrents"`
}

type StringedGroup struct {
	WikiBody            string          `json:"wikiBody"`
	WikiImage           string          `json:"wikiImage"`
	ID                  int             `json:"id,string"`
	Name                string          `json:"name"`
	Year                int             `json:"year,string"`
	RecordLabel         string          `json:"recordLabel"`
	CatalogueNumber     string          `json:"catalogueNumber"`
	ReleaseType         int             `json:"releaseType,string"`
	CategoryID          int             `json:"categoryId,string"`
	CategoryName        string          `json:"categoryName"`
	Time                string          `json:"time"`
	VanityHouse         bool            `json:"-"` // damn string response
	MusicInfo           model.MusicInfo `json:"musicInfo" sql:"-"`
	MusicInfoSerialized []byte          `json:"-" gorm:"type:text"`
	Tags                []string        `json:"tags" sql:"-"`
	TagsSerialized      []byte          `json:"-" gorm:"type:text"`
}

// The Gazelle Collage-API returns all torrent groups with their attributes
// cast to string which causes lots of problems for Go's strict typing.
// So we redefine the type here and fixup the fields manually by parsing them
// as ",string".
// Sadly, vanityHouse is a stringly-typed numeric boolean - a case which the
// Go encoder does not handle at all. I don't want to write a 20 line workaround
// for this shit, so we just skip the field. Bye.
func (g StringedGroup) Fix() model.Group {
	return model.Group(g)
}

func (cg CollageWithStringedGroups) Fix() model.CollageWithGroups {
	cf := model.CollageWithGroups{Collage: cg.Collage}
	for _, gt := range cg.TorrentGroups {
		cf.TorrentGroups = append(cf.TorrentGroups, model.GroupWithTorrents{
			gt.StringedGroup.Fix(),
			gt.Torrents,
		})
	}
	return cf
}
