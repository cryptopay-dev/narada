package narada

import (
	"fmt"
	"reflect"

	"github.com/spf13/viper"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
)

var (
	sentryLevelMap = map[logrus.Level]sentry.Level{
		logrus.TraceLevel: sentry.LevelDebug,
		logrus.DebugLevel: sentry.LevelDebug,
		logrus.InfoLevel:  sentry.LevelInfo,
		logrus.WarnLevel:  sentry.LevelWarning,
		logrus.ErrorLevel: sentry.LevelError,
		logrus.FatalLevel: sentry.LevelFatal,
		logrus.PanicLevel: sentry.LevelFatal,
	}
)

type LogrusSentryHook struct {
}

func NewLogrusSentryHook() LogrusSentryHook {
	return LogrusSentryHook{}
}

func (h LogrusSentryHook) Fire(entry *logrus.Entry) error {
	sentry.CaptureEvent(h.mapEntry(entry))

	return nil
}

func (h LogrusSentryHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
}

func (h LogrusSentryHook) mapEntry(entry *logrus.Entry) *sentry.Event {
	event := sentry.NewEvent()
	event.Message = entry.Message
	event.Level = sentryLevelMap[entry.Level]

	for k, v := range entry.Data {
		event.Extra[k] = v
	}

	if err, ok := entry.Data[logrus.ErrorKey].(error); ok {
		exception := sentry.Exception{
			Type:       reflect.TypeOf(err).String(),
			Value:      err.Error(),
			Stacktrace: sentry.ExtractStacktrace(err),
		}

		event.Exception = []sentry.Exception{exception}
	}

	return event
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

	go c.SendMessage(msg) //nolint:errcheck

	return nil
}

func (h LogrusSlackHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
}
