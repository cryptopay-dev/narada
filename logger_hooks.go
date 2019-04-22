package tuktuk

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"

	"github.com/getsentry/raven-go"
	"github.com/sirupsen/logrus"
)

type LogrusSentryHook struct {
	client *raven.Client
}

func NewLogrusSentryHook(client *raven.Client) LogrusSentryHook {
	return LogrusSentryHook{
		client: client,
	}
}

func (h LogrusSentryHook) Fire(entry *logrus.Entry) error {
	var notifyErr error
	err, ok := entry.Data["error"].(error)
	if ok {
		notifyErr = err
	} else {
		notifyErr = errors.New(entry.Message)
	}

	h.client.CaptureError(notifyErr, nil)

	return nil
}

func (h LogrusSentryHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
}

type LogrusSlackHook struct {
	url      string
	icon     string
	channel  string
	emoji    string
	username string
	extra    map[string]interface{}
	enabled  bool
}

func NewLogrusSlackHook(config *viper.Viper) LogrusSlackHook {
	config.SetDefault("logger.slack", false)
	config.SetDefault("logger.slack_url", "")
	config.SetDefault("logger.slack_icon", "")
	config.SetDefault("logger.slack_emoji", ":ghost:")
	config.SetDefault("logger.slack_username", config.GetString("app.name")+"_bot")

	// Binding extra fields
	extra := make(map[string]interface{})
	extra["app_name"] = config.GetString("app.name")
	extra["app_version"] = config.GetString("app.version")

	return LogrusSlackHook{
		url:      config.GetString("logger.slack_url"),
		icon:     config.GetString("logger.slack_icon"),
		channel:  config.GetString("logger.slack_channel"),
		emoji:    config.GetString("logger.slack_emoji"),
		username: config.GetString("logger.slack_username"),
		enabled:  config.GetBool("logger.slack"),
		extra:    config.GetStringMap("logger.slack_extra"),
	}
}

func (h LogrusSlackHook) Fire(entry *logrus.Entry) error {
	if !h.enabled {
		return nil
	}

	msg := &SlackMessage{
		Username:  h.username,
		Channel:   h.channel,
		IconEmoji: h.emoji,
		IconUrl:   h.icon,
	}

	data := map[string]interface{}{}

	for k, v := range h.extra {
		data[k] = v
	}
	for k, v := range entry.Data {
		data[k] = v
	}

	newEntry := &logrus.Entry{
		Logger:  entry.Logger,
		Data:    data,
		Time:    entry.Time,
		Level:   entry.Level,
		Message: entry.Message,
	}

	attach := NewAttachment()
	if len(newEntry.Data) > 0 {
		// Add a header above field data
		attach.Text = "Message fields"

		for k, v := range newEntry.Data {
			slackField := NewField()

			slackField.Title = k
			slackField.Value = fmt.Sprint(v)
			// If the field is <= 20 then we'll set it to short
			if len(slackField.Value) <= 20 {
				slackField.Short = true
			}

			attach.AddField(slackField)
		}
		attach.Pretext = newEntry.Message
	} else {
		attach.Text = newEntry.Message
	}
	attach.Fallback = newEntry.Message
	attach.Color = "danger"

	msg.AddAttachment(attach)

	c := NewClient(h.url)

	go c.SendMessage(msg)

	return nil
}

func (h LogrusSlackHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
}
