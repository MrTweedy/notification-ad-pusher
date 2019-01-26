package main

import (
	"database/sql"
	"net/http"
	auth "github.com/abbot/go-http-auth"
)

var configVars = make(map[string]string)
var db sql.DB
var errLog []string
var sendCounter int
var prop string
var mode string
var myClient http.Client
var monitorOutput MessageQue
var locked bool
var abort bool
var authTokens []string

type userRecord struct {
	OSid         string
	Device_model string
	Device_type  string
	Device_os    string
	IP           string
}
type AdObject struct {
	App_id             string            `json:"app_id"`
	Include_player_ids [1]string         `json:"include_player_ids"`
	Contents           map[string]string `json:"contents"`
	Headings           map[string]string `json:"headings"`
	Url                string            `json:"url"`
	Small_icon         string            `json:"small_icon"`
	Large_icon         string            `json:"large_icon"`
	Adm_small_icon     string            `json:"adm_small_icon"`
	Adm_large_icon     string            `json:"adm_large_icon"`
	Chrome_web_icon    string            `json:"chrome_web_icon"`
}
type OSargs struct {
	App_id       string   `json:"app_id"`
	Extra_fields []string `json:"extra_fields"`
}

var authenticator *auth.DigestAuth = auth.NewDigestAuthenticator("example.com", func(user, realm string) string{
		if user == "john" {
			// password is "hello"
			return "b98e16cbc3d01734b264adba7baa3bf9"
		}
		return ""
})

type MessageQue struct {
  que []string
}
func (this *MessageQue) unshift(input string, noDupe ...bool) {
  if len(noDupe) == 1 && noDupe[0] == true {
    if len(this.check(1)) == 0 || (len(this.check(1)) == 1 && this.check(1)[0] != input) {
      this.que = append([]string{input}, this.que...)
    }
  } else {
    this.que = append([]string{input}, this.que...)
  }
}
func (this *MessageQue) pop(single ...bool) (output string) {
  if len(this.que) > 0 {
    output = this.que[ len(this.que) - 1 ]
    this.que = this.que[ : len(this.que) - 1]
  } else {
    output = ""
  }
  return
}
func (this *MessageQue) check(i int) (output []string) {
  if i > len(this.que){
    i = len(this.que)
  } else if i < 0 {
    i = 0
  }
  output = this.que[ : i ]
  return
}
