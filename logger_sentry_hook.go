package tuktuk

import (
	"errors"

	"github.com/getsentry/raven-go"
	"github.com/sirupsen/logrus"
)

type LogrusSentryHook struct {
	client *raven.Client
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

func NewLogrusSentryHook(client *raven.Client) LogrusSentryHook {
	return LogrusSentryHook{
		client: client,
	}
}

func (h LogrusSentryHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
}
