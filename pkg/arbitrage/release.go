// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package arbitrage

import (
	"crypto/sha1"
	"encoding/base32"
	"encoding/json"
	"errors"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Release struct {
	Id       int64  `json:"-"`
	SourceId string `json:"source_id" gorm:"index:"`

	ListHash string `json:"list_hash" gorm:"index"`
	NameHash string `json:"name_hash" gorm:"index"`
	SizeHash string `json:"size_hash" gorm:"index"`

	FileList string `json:"fileList" gorm:"type:text"`
	FilePath string `json:"filePath" gorm:"type:text"`
	Time     string `json:"time"`
}

type File struct {
	Name string
	Size int64
}

func FromResponse(resp *Response) (*Release, error) {
	r := &Release{}
	return r, r.FromResponse(resp)
}

func (r *Release) FromResponse(resp *Response) error {
	if resp.Type != "torrent" {
		return errors.New("Expected response of type 'torrent' not: " + resp.Type)
	}
	r.SourceId = resp.Source + ":" + strconv.Itoa(resp.TypeId)

	raw := json.RawMessage(resp.Response)
	sections := make(map[string]*json.RawMessage)
	if err := json.Unmarshal(raw, &sections); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(*sections["torrent"]), r); err != nil {
		return err
	}
	files := ParseFileList(html.UnescapeString(r.FileList))
	r.FileList = FilesToList(files)
	r.FilePath = html.UnescapeString(r.FilePath)
	r.CalculateHashes()
	return nil
}

func hash(in string) string {
	hash := sha1.Sum([]byte(in))
	slice := hash[0:len(hash)]
	return base32.StdEncoding.EncodeToString(slice)
}

func (r *Release) CalculateHashes() {
	r.ListHash = "FL-" + hash(r.FileList)
	r.NameHash = "DN-" + hash(r.FilePath)

	sizeHash := ""
	for _, f := range ParseFileList(r.FileList) {
		sizeHash += strconv.FormatInt(f.Size, 10) + "|"
	}
	r.SizeHash = "FS-" + hash(strings.TrimRight(sizeHash, "|"))
}

func FromFile(root string) (*Release, error) {
	files := make([]File, 0)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			path, _ = filepath.Rel(root, path)
			files = append(files, File{
				Name: path,
				Size: info.Size(),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	r := &Release{
		FilePath: filepath.Base(root),
		FileList: FilesToList(files),
	}
	r.CalculateHashes()
	r.SourceId = "file:" + r.ListHash
	return r, nil
}

func ParseFileList(filestr string) []File {
	if filestr == "" {
		return []File{}
	}
	lines := strings.Split(filestr, "|||")
	files := make([]File, len(lines))
	for i, l := range lines {
		parts := strings.SplitN(l, "{{{", 2)
		files[i].Name = parts[0]
		size, _ := strconv.ParseInt(strings.TrimRight(parts[1], "}"), 10, 64)
		files[i].Size = size
	}
	sort.Sort(ByName(files))
	return files
}

type ByName []File

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

func FilesToList(files []File) string {
	sort.Sort(ByName(files))
	list := ""
	for _, f := range files {
		if strings.HasSuffix(f.Name, "release.info.yaml") {
			continue
		}
		list += f.Name + "{{{" + strconv.FormatInt(f.Size, 10) + "}}}|||"
	}
	return strings.TrimRight(list, "|")
}
