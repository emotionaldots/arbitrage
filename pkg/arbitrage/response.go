// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package arbitrage

import "time"

type Response struct {
	Id       int64     `json:"-"`
	Source   string    `json:"source"`
	Type     string    `json:"type"`
	TypeId   int       `json:"type_id"`
	Response string    `gorm:"type:text"`
	Time     time.Time `json:"time"`
}
