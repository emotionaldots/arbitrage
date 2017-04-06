// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package arbitrage

import (
	"fmt"
	"strconv"
	"time"
)

type Response struct {
	Id         int64     `json:"-"`
	Source     string    `json:"source"`
	Type       string    `json:"type"`
	Identifier string    `json:"string"`
	TypeId     int       `json:"type_id"`
	Response   string    `gorm:"type:text"`
	Time       time.Time `json:"time"`
}

func (resp *Response) UID() string {
	return Pad(resp.TypeId) + "|" + resp.Time.UTC().Format(time.RFC3339)
}

func (resp Response) String() string {
	if resp.TypeId != 0 {
		return resp.Source + ":" + resp.Type + ":" + strconv.Itoa(resp.TypeId)
	}
	return resp.Source + ":" + resp.Type + ":" + resp.Identifier
}

func Pad(id int) string {
	return fmt.Sprintf("%010d", id)
}
