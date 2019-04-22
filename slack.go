package tuktuk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type (
	SlackClient struct {
		Url string
	}

	SlackMessage struct {
		Text        string             `json:"text"`
		Username    string             `json:"username"`
		IconUrl     string             `json:"icon_url"`
		IconEmoji   string             `json:"icon_emoji"`
		Channel     string             `json:"channel"`
		UnfurlLinks bool               `json:"unfurl_links"`
		Attachments []*SlackAttachment `json:"attachments"`
	}

	SlackAttachment struct {
		Title    string        `json:"title"`
		Fallback string        `json:"fallback"`
		Text     string        `json:"text"`
		Pretext  string        `json:"pretext"`
		Color    string        `json:"color"`
		Fields   []*SlackField `json:"fields"`
	}

	SlackField struct {
		Title string `json:"title"`
		Value string `json:"value"`
		Short bool   `json:"short"`
	}

	SlackError struct {
		Code int
		Body string
	}
)

func (e *SlackError) Error() string {
	return fmt.Sprintf("SlackError: %d %s", e.Code, e.Body)
}

func NewClient(url string) *SlackClient {
	return &SlackClient{url}
}

func (c *SlackClient) SendMessage(msg *SlackMessage) error {
	body, _ := json.Marshal(msg)
	buf := bytes.NewReader(body)

	resp, err := http.Post(c.Url, "application/json", buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t, _ := ioutil.ReadAll(resp.Body)
		return &SlackError{resp.StatusCode, string(t)}
	}

	return nil
}

func NewAttachment() *SlackAttachment {
	return &SlackAttachment{}
}

func (m *SlackMessage) AddAttachment(a *SlackAttachment) {
	m.Attachments = append(m.Attachments, a)
}

func NewField() *SlackField {
	return &SlackField{}
}

func (a *SlackAttachment) AddField(f *SlackField) {
	a.Fields = append(a.Fields, f)
}
