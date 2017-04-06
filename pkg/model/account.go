package model

type Account struct {
	Username      string `json:"username"`
	ID            int    `json:"id"`
	AuthKey       string `json:"authKey"`
	PassKey       string `json:"passKey"`
	Notifications struct {
		Messages       int  `json:"messages"`
		Notifications  int  `json:"notifications"`
		NewAnnouncment bool `json:"newAnnouncment"`
		NewBlog        bool `json:"newBlog"`
	} `json:"notifications"`
	UserStats struct {
		Uploaded      int64   `json:"uploaded"`
		Downloaded    int64   `json:"downloaded"`
		Ratio         float64 `json:"ratio"`
		RequiredRatio float64 `json:"requiredRatio"`
		Class         string  `json:"class"`
	} `json:"userstats"`
}
