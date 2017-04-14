package torrentinfo

import (
	"io"
	"os"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

type MetaInfo struct {
	InfoBytes    bencode.Bytes         `bencode:"info"`
	Announce     string                `bencode:"announce,omitempty"`
	AnnounceList metainfo.AnnounceList `bencode:"announce-list,omitempty"`
	CreationDate int64                 `bencode:"creation date,omitempty"`
	Comment      string                `bencode:"comment,omitempty"`
	CreatedBy    string                `bencode:"created by,omitempty"`
	Encoding     string                `bencode:"encoding,omitempty"`
}

// Load a MetaInfo from an io.Reader. Returns a non-nil error in case of
// failure.
func Load(r io.Reader) (*MetaInfo, error) {
	var mi MetaInfo
	d := bencode.NewDecoder(r)
	err := d.Decode(&mi)
	if err != nil {
		return nil, err
	}
	return &mi, nil
}

// Convenience function for loading a MetaInfo from a file.
func LoadFromFile(filename string) (*MetaInfo, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

func (mi MetaInfo) UnmarshalInfo() (info metainfo.Info, err error) {
	err = bencode.Unmarshal(mi.InfoBytes, &info)
	return
}
