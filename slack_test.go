package narada

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

func TestNewClient(t *testing.T) {
	gock.Clean()

	// Reading fixture
	buf, err := ioutil.ReadFile("./fixtures/slack/request.json")
	assert.NoError(t, err)

	t.Run("Errors", func(t *testing.T) {
		t.Run("Bad url", func(t *testing.T) {
			client := NewClient("i am a bad url!")
			msg := &SlackMessage{}

			err := client.SendMessage(msg)
			assert.Error(t, err)
		})

		t.Run("Bad response code", func(t *testing.T) {
			defer gock.Off()
			gock.New("http://slack_404_testing.slack.com").
				Post("/").
				Reply(404)

			client := NewClient("http://slack_404_testing.slack.com")
			msg := &SlackMessage{}

			err := client.SendMessage(msg)
			assert.Error(t, err)
		})
	})

	t.Run("Success", func(t *testing.T) {
		defer gock.Off()
		gock.New("http://slack_testing.slack.com").
			Post("/").
			MatchType("json").
			BodyString(string(buf)).
			Reply(200)

		client := NewClient("http://slack_testing.slack.com")
		assert.NotNil(t, client)

		msg := &SlackMessage{
			Text:        "Hello",
			Username:    "Bot",
			IconEmoji:   "icon-emoji",
			IconUrl:     "icon-url",
			Channel:     "general",
			UnfurlLinks: false,
		}

		attach := NewAttachment()
		attach.Text = "Attachment #1"
		attach.Color = "red"
		attach.Fallback = "http://google.com"
		attach.Pretext = "pre text"
		attach.Title = "Attachment title"

		field1 := NewField()
		field1.Title = "Field 1"
		field1.Value = "Field 1 - Value"
		field1.Short = true

		field2 := NewField()
		field2.Title = "Field 2"
		field2.Value = "Field 2 - Value"
		field2.Short = false

		attach.AddField(field1)
		attach.AddField(field2)

		msg.AddAttachment(attach)

		{
			err := client.SendMessage(msg)
			assert.NoError(t, err)
		}
	})
}

func TestMessage(t *testing.T) {
	message := SlackMessage{}
	assert.Equal(t, 0, len(message.Attachments))

	attachment := NewAttachment()
	message.AddAttachment(attachment)
	assert.Equal(t, 1, len(message.Attachments))
}

func TestSlackError_Error(t *testing.T) {
	err := SlackError{
		Code: 100,
		Body: "Error body",
	}

	assert.Equal(t, "SlackError: 100 Error body", err.Error())
}
