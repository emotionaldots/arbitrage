// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package arbitrage

import (
	"crypto/sha1"
	"encoding/base32"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Release struct {
	Id       int64  `json:"-"`
	Source   string `json:"source"`
	SourceId int64  `json:"source_id" gorm:"index"`

	HashType string `json:"hash_type"`
	Hash     string `json:"hash" gorm:"index"`

	FileList []File `json:"fileList,omitempty" sql:"-"`
	FilePath string `json:"filePath" gorm:"type:text"`
	Time     string `json:"time"`
}

type File struct {
	Name string
	Size int64
}

func hash(in string) string {
	hash := sha1.Sum([]byte(in))
	slice := hash[0:len(hash)]
	return base32.StdEncoding.EncodeToString(slice)
}

func HashFileList(files []File) string {
	return "FL-" + hash(FilesToList(files))
}

var reExtensions = regexp.MustCompile(`\.(epub|mobi|mp3|flac|mkv|avi|log)$`)

func HashReducedList(files []File) string {
	selected := make([]File, 0)
	for _, f := range files {
		if !reExtensions.MatchString(f.Name) {
			continue
		}
		f.Size = RoundBytes(f.Size)
		selected = append(selected, f)
	}
	if len(selected) == 0 {
		for _, f := range files {
			f.Size = RoundBytes(f.Size)
			selected = append(selected, f)
		}
	}
	if len(selected) == 0 {
		return ""
	}
	return "RL-" + hash(FilesToList(selected))
}

func HashDefault(r *Release) {
	r.HashType = "RL"
	r.Hash = HashReducedList(r.FileList)
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
		FileList: files,
	}
	r.Source = "file"
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
		if len(parts) == 1 {
			continue
		}
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
