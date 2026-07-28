package narada

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/viper"
)

// reportLevel is the threshold at or above which a record is forwarded to the external sinks (Sentry, Slack).
// It matches the logrus hooks it replaces, which registered for the error/fatal/panic levels only.
const reportLevel = slog.LevelError

// attrSet accumulates the attributes added through WithAttrs/WithGroup so a decorating handler can report them alongside a record's own attributes.
// Group-qualified keys are flattened as "group.key", mirroring how the text handler renders them.
type attrSet struct {
	fields map[string]any
	prefix string
}

func (s attrSet) with(attrs []slog.Attr) attrSet {
	if len(attrs) == 0 {
		return s
	}

	next := attrSet{
		fields: make(map[string]any, len(s.fields)+len(attrs)),
		prefix: s.prefix,
	}
	maps.Copy(next.fields, s.fields)

	for _, a := range attrs {
		flattenAttr(next.fields, s.prefix, a)
	}

	return next
}

func (s attrSet) withGroup(name string) attrSet {
	if name == "" {
		return s
	}

	return attrSet{fields: s.fields, prefix: s.prefix + name + "."}
}

// merge returns the accumulated attributes plus the record's own.
func (s attrSet) merge(r slog.Record) map[string]any {
	out := make(map[string]any, len(s.fields)+r.NumAttrs())
	maps.Copy(out, s.fields)

	r.Attrs(func(a slog.Attr) bool {
		flattenAttr(out, s.prefix, a)
		return true
	})

	return out
}

func flattenAttr(dst map[string]any, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()

	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "."
		}
		for _, ga := range a.Value.Group() {
			flattenAttr(dst, prefix, ga)
		}

		return
	}

	// slog drops empty attributes; so do we.
	if a.Equal(slog.Attr{}) {
		return
	}

	dst[prefix+a.Key] = a.Value.Any()
}

// extractError returns the error carried under ErrorKey, or a new error built from the record message when the record carries none.
func extractError(fields map[string]any, msg string) error {
	if err, ok := fields[ErrorKey].(error); ok {
		return err
	}

	return errors.New(msg)
}

func sentryLevel(l slog.Level) sentry.Level {
	switch {
	case l < slog.LevelInfo:
		return sentry.LevelDebug
	case l < slog.LevelWarn:
		return sentry.LevelInfo
	case l < slog.LevelError:
		return sentry.LevelWarning
	default:
		return sentry.LevelError
	}
}

// SentryHandler reports error-level records to Sentry and forwards every record to the next handler.
type SentryHandler struct {
	next  slog.Handler
	attrs attrSet
}

var _ slog.Handler = (*SentryHandler)(nil)

func NewSentryHandler(next slog.Handler) *SentryHandler {
	return &SentryHandler{next: next}
}

func (h *SentryHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *SentryHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= reportLevel {
		data := h.attrs.merge(r)

		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentryLevel(r.Level))
			scope.SetContext("data", data)

			sentry.CaptureException(extractError(data, r.Message))
		})
	}

	return h.next.Handle(ctx, r)
}

func (h *SentryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	return &SentryHandler{next: h.next.WithAttrs(attrs), attrs: h.attrs.with(attrs)}
}

func (h *SentryHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	return &SentryHandler{next: h.next.WithGroup(name), attrs: h.attrs.withGroup(name)}
}

// SlackHandler posts error-level records to a Slack webhook and forwards every
// record to the next handler.
type SlackHandler struct {
	next  slog.Handler
	attrs attrSet

	url      string
	icon     string
	channel  string
	emoji    string
	username string
	extra    map[string]any
	enabled  bool
}

var _ slog.Handler = (*SlackHandler)(nil)

func NewSlackHandler(config *viper.Viper, next slog.Handler) *SlackHandler {
	config.SetDefault("logger.slack", false)
	config.SetDefault("logger.slack_url", "")
	config.SetDefault("logger.slack_icon", "")
	config.SetDefault("logger.slack_emoji", ":ghost:")
	config.SetDefault("logger.slack_username", config.GetString("app.name")+"_bot")

	// Binding extra fields. The configured extras win over the app defaults.
	extra := make(map[string]any)
	extra["app_name"] = config.GetString("app.name")
	extra["app_version"] = config.GetString("app.version")
	maps.Copy(extra, config.GetStringMap("logger.slack_extra"))

	return &SlackHandler{
		next:     next,
		url:      config.GetString("logger.slack_url"),
		icon:     config.GetString("logger.slack_icon"),
		channel:  config.GetString("logger.slack_channel"),
		emoji:    config.GetString("logger.slack_emoji"),
		username: config.GetString("logger.slack_username"),
		enabled:  config.GetBool("logger.slack"),
		extra:    extra,
	}
}

func (h *SlackHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *SlackHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.enabled && r.Level >= reportLevel {
		h.post(r)
	}

	return h.next.Handle(ctx, r)
}

func (h *SlackHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	next := *h
	next.next = h.next.WithAttrs(attrs)
	next.attrs = h.attrs.with(attrs)

	return &next
}

func (h *SlackHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	next := *h
	next.next = h.next.WithGroup(name)
	next.attrs = h.attrs.withGroup(name)

	return &next
}

func (h *SlackHandler) post(r slog.Record) {
	msg := &SlackMessage{
		Username:  h.username,
		Channel:   h.channel,
		IconEmoji: h.emoji,
		IconUrl:   h.icon,
	}

	data := h.attrs.merge(r)
	for k, v := range h.extra {
		if _, ok := data[k]; !ok {
			data[k] = v
		}
	}

	attach := NewAttachment()
	if len(data) > 0 {
		// Add a header above field data
		attach.Text = "Message fields"

		for k, v := range data {
			slackField := NewField()

			slackField.Title = k
			slackField.Value = fmt.Sprint(v)
			// If the field is <= 20 then we'll set it to short
			if len(slackField.Value) <= 20 {
				slackField.Short = true
			}

			attach.AddField(slackField)
		}
		attach.Pretext = r.Message
	} else {
		attach.Text = r.Message
	}
	attach.Fallback = r.Message
	attach.Color = "danger"

	msg.AddAttachment(attach)

	c := NewClient(h.url)

	go c.SendMessage(msg) //nolint:errcheck
}
