package model

import "fmt"

type Torrent struct {
	ID                      int    `json:"id"`
	GroupID                 int    `json:"groupID"`
	Media                   string `json:"media"`
	Format                  string `json:"format"`
	Encoding                string `json:"encoding"`
	Remastered              bool   `json:"remastered"`
	RemasterYear            int    `json:"remasterYear"`
	RemasterTitle           string `json:"remasterTitle"`
	RemasterRecordLabel     string `json:"remasterRecordLabel"`
	RemasterCatalogueNumber string `json:"remasterCatalogueNumber"`
	Scene                   bool   `json:"scene"`
	HasLog                  bool   `json:"hasLog"`
	HasCue                  bool   `json:"hasCue"`
	LogScore                int    `json:"logScore"`
	FileCount               int    `json:"fileCount"`
	Size                    int    `json:"size"`
	Seeders                 int    `json:"seeders"`
	Leechers                int    `json:"leechers"`
	Snatched                int    `json:"snatched"`
	FreeTorrent             bool   `json:"freeTorrent"`
	Time                    string `json:"time"`
	Description             string `json:"description"`
	FileList                string `json:"fileList"`
	FilePath                string `json:"filePath"`
	UserID                  int    `json:"userID"`
	Username                string `json:"username"`
}

func (t Torrent) String() string {
	return fmt.Sprintf("torrent %d: %s [%s-%s]", t.ID, t.FilePath, t.Media, t.Format)
}
