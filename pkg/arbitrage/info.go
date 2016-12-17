// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package arbitrage

import "time"

type Info struct {
	Version     int                    `yaml:"version"`
	FileHash    string                 `yaml:"file_hash"`
	LastUpdated time.Time              `yaml:"last_updated"`
	Releases    map[string]InfoRelease `yaml:"releases,omitempty"`
}

type InfoRelease struct {
	TorrentId int    `yaml:"torrent_id"`
	Format    string `yaml:"format"`
	FilePath  string `yaml:"file_path"`

	Name            string `yaml:"name"`
	Year            int    `yaml:"year"`
	RecordLabel     string `yaml:"record_label"`
	CatalogueNumber string `yaml:"catalogue_number"`
	Edition         string `yaml:"edition"`

	Composers []string `yaml:"composers,omitempty,flow"`
	Artists   []string `yaml:"artists"`
	With      []string `yaml:"with,omitempty,flow"`
	DJ        []string `yaml:"dj,omitempty,flow"`
	RemixedBy []string `yaml:"remixed_by,omitempty,flow"`
	Producer  []string `yaml:"producer,omitempty,flow"`

	Tags        []string `yaml:"tags,flow"`
	Description string   `yaml:"description,omitempty"`
	Image       string   `yaml:"image,omitempty"`
}

func concat(s, del, extra, pre, suf string) string {
	if extra == "" {
		return s
	}
	if s != "" {
		s += del
	}
	return s + pre + extra + suf
}

func (i InfoRelease) String() string {
	str := ""
	if len(i.Artists) == 1 {
		str = i.Artists[0]
	} else if len(i.Artists) > 1 {
		str = "Various Artists"
	} else {
		str = "Unknown Artist"
	}

	str = concat(str, " - ", i.Name, "", "")
	str = concat(str, " ", i.Format, "[", "]")
	str = concat(str, " ", i.CatalogueNumber, "{", "}")
	return str
}
