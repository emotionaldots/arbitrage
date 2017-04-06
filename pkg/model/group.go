package model

import "encoding/json"

type Group struct {
	WikiBody            string    `json:"wikiBody"`
	WikiImage           string    `json:"wikiImage"`
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	Year                int       `json:"year"`
	RecordLabel         string    `json:"recordLabel"`
	CatalogueNumber     string    `json:"catalogueNumber"`
	ReleaseType         int       `json:"releaseType"`
	CategoryID          int       `json:"caregoryId"`
	CategoryName        string    `json:"categoryName"`
	Time                string    `json:"time"`
	VanityHouse         bool      `json:"vanityHouse"`
	MusicInfo           MusicInfo `json:"musicInfo" sql:"-"`
	MusicInfoSerialized []byte    `json:"-" gorm:"type:text"`
	Tags                []string  `json:"tags" sql:"-"`
	TagsSerialized      []byte    `json:"-" gorm:"type:text"`
}

func (g *Group) BeforeSave() (err error) {
	if len(g.Tags) > 0 {
		g.TagsSerialized, err = json.Marshal(g.Tags)
		if err != nil {
			return err
		}
	} else {
		g.TagsSerialized = nil
	}
	g.MusicInfoSerialized, err = json.Marshal(g.MusicInfo)
	return err
}

type MusicInfo struct {
	Composers []ArtistLink `json:"composers"`
	DJ        []ArtistLink `json:"dj"`
	Artists   []ArtistLink `json:"artists"`
	With      []ArtistLink `json:"with"`
	Conductor []ArtistLink `json:"conductor"`
	RemixedBy []ArtistLink `json:"remixedBy"`
	Producer  []ArtistLink `json:"producer"`
}

type ArtistLink struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
