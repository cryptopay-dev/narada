package narada

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/viper"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// discardTextHandler is a real handler that writes nowhere. Unlike
// slog.DiscardHandler its Enabled reports true, so records actually reach the
// handler under test.
func discardTextHandler() slog.Handler {
	return slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
}

// slackServer stands in for the Slack webhook. Every decoded POST body is
// published on the returned channel, so a test can assert that a message really
// was sent — and what was in it.
func slackServer(t *testing.T) (*httptest.Server, <-chan SlackMessage) {
	t.Helper()

	posts := make(chan SlackMessage, 4)

	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		var msg SlackMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Errorf("decoding slack payload: %v", err)
		}
		posts <- msg

		rw.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	return srv, posts
}

func slackConfig(url string) *viper.Viper {
	cfg := viper.New()
	cfg.Set("logger.slack", true)
	cfg.Set("logger.slack_url", url)
	cfg.Set("logger.slack_channel", "general")
	cfg.Set("app.name", "default")

	return cfg
}

func awaitPost(t *testing.T, posts <-chan SlackMessage) SlackMessage {
	t.Helper()

	select {
	case msg := <-posts:
		return msg
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout exceeded")
		return SlackMessage{}
	}
}

func slackFields(a *SlackAttachment) map[string]string {
	fields := make(map[string]string, len(a.Fields))
	for _, f := range a.Fields {
		fields[f.Title] = f.Value
	}

	return fields
}

func errorRecord(msg string, attrs ...slog.Attr) slog.Record {
	r := slog.NewRecord(time.Now(), slog.LevelError, msg, 0)
	r.AddAttrs(attrs...)

	return r
}

func TestSentryHandler(t *testing.T) {
	t.Run("It should work with empty sentry", func(t *testing.T) {
		t.Run("Sending error", func(t *testing.T) {
			h := NewSentryHandler(discardTextHandler())
			assert.NotNil(t, h)
			assert.NoError(t, h.Handle(
				context.Background(),
				errorRecord("failed", Err(errors.New("some unknown error"))),
			))
		})

		t.Run("Sending message", func(t *testing.T) {
			h := NewSentryHandler(discardTextHandler())
			assert.NotNil(t, h)
			assert.NoError(t, h.Handle(context.Background(), errorRecord("i am unknown message")))
		})
	})

	t.Run("It reports only at error level and above", func(t *testing.T) {
		h := NewSentryHandler(discardTextHandler())

		for _, lvl := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn} {
			r := slog.NewRecord(time.Now(), lvl, "not reported", 0)
			assert.NoError(t, h.Handle(context.Background(), r))
		}
	})

	t.Run("It carries attributes and groups through", func(t *testing.T) {
		h := NewSentryHandler(discardTextHandler()).
			WithAttrs([]slog.Attr{slog.String("module", "db")}).
			WithGroup("req").
			WithAttrs([]slog.Attr{slog.Int("status", 500)})

		assert.NoError(t, h.Handle(context.Background(), errorRecord("boom")))

		// Degenerate cases must not allocate a new handler.
		assert.Same(t, h, h.WithAttrs(nil))
		assert.Same(t, h, h.WithGroup(""))
	})

	t.Run("Enabled delegates to the next handler", func(t *testing.T) {
		h := NewSentryHandler(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))

		assert.False(t, h.Enabled(context.Background(), slog.LevelInfo))
		assert.True(t, h.Enabled(context.Background(), slog.LevelError))
	})
}

func TestSentryLevel(t *testing.T) {
	assert.Equal(t, sentry.LevelDebug, sentryLevel(slog.LevelDebug))
	assert.Equal(t, sentry.LevelInfo, sentryLevel(slog.LevelInfo))
	assert.Equal(t, sentry.LevelWarning, sentryLevel(slog.LevelWarn))
	assert.Equal(t, sentry.LevelError, sentryLevel(slog.LevelError))
}

func TestExtractError(t *testing.T) {
	wrapped := errors.New("wrapped")

	assert.Equal(t, wrapped, extractError(map[string]any{ErrorKey: wrapped}, "message"))
	assert.EqualError(t, extractError(map[string]any{}, "message"), "message")
	// A non-error value under ErrorKey must not be mistaken for one.
	assert.EqualError(t, extractError(map[string]any{ErrorKey: "oops"}, "message"), "message")
}

func TestAttrSet(t *testing.T) {
	s := attrSet{}.
		with([]slog.Attr{slog.String("module", "db")}).
		withGroup("req").
		with([]slog.Attr{
			slog.Int("status", 500),
			slog.Group("client", slog.String("ip", "127.0.0.1")),
			{}, // empty attrs are dropped, as slog does
		})

	r := errorRecord("boom", slog.String("phase", "commit"))

	assert.Equal(t, map[string]any{
		"module":        "db",
		"req.status":    int64(500),
		"req.client.ip": "127.0.0.1",
		"req.phase":     "commit",
	}, s.merge(r))

	// with()/withGroup() are no-ops for empty input and never mutate the receiver.
	assert.Equal(t, s, s.with(nil))
	assert.Equal(t, s, s.withGroup(""))
}

func TestSlackHandler(t *testing.T) {
	t.Run("Disabled by default", func(t *testing.T) {
		h := NewSlackHandler(viper.New(), discardTextHandler())
		assert.NoError(t, h.Handle(context.Background(), errorRecord("nothing is sent")))
	})

	t.Run("Sending message to Slack", func(t *testing.T) {
		t.Run("With error attached", func(t *testing.T) {
			srv, posts := slackServer(t)

			cfg := slackConfig(srv.URL)
			cfg.Set("logger.slack_extra", map[string]any{
				"question": 42,
			})

			logger := slog.New(NewSlackHandler(cfg, discardTextHandler()))
			logger.Error("error for slack", Err(errors.New("unknown error")))

			msg := awaitPost(t, posts)
			assert.Equal(t, "general", msg.Channel)
			assert.Equal(t, "default_bot", msg.Username)
			require.Len(t, msg.Attachments, 1)

			attach := msg.Attachments[0]
			assert.Equal(t, "error for slack", attach.Fallback)
			assert.Equal(t, "error for slack", attach.Pretext)
			assert.Equal(t, "danger", attach.Color)
			assert.Equal(t, map[string]string{
				"error":       "unknown error",
				"question":    "42",
				"app_name":    "default",
				"app_version": "",
			}, slackFields(attach))
		})

		t.Run("With error only in message", func(t *testing.T) {
			srv, posts := slackServer(t)

			logger := slog.New(NewSlackHandler(slackConfig(srv.URL), discardTextHandler()))
			logger.Error("error for slack: unknown error")

			msg := awaitPost(t, posts)
			require.Len(t, msg.Attachments, 1)
			assert.Equal(t, "error for slack: unknown error", msg.Attachments[0].Fallback)
		})
	})

	t.Run("Below the report level nothing is posted", func(t *testing.T) {
		srv, posts := slackServer(t)

		logger := slog.New(NewSlackHandler(slackConfig(srv.URL), discardTextHandler()))
		logger.Debug("not important enough")
		logger.Info("not important enough")
		logger.Warn("not important enough")

		// post() dispatches immediately, so a short grace period is enough to
		// show that nothing was sent.
		select {
		case <-posts:
			t.Fatal("a sub-error record was posted to Slack")
		case <-time.After(200 * time.Millisecond):
		}
	})

	t.Run("It carries attributes and groups through", func(t *testing.T) {
		h := NewSlackHandler(viper.New(), discardTextHandler())

		withAttrs := h.WithAttrs([]slog.Attr{slog.String("module", "db")})
		assert.NotSame(t, h, withAttrs)
		assert.NoError(t, withAttrs.WithGroup("req").Handle(context.Background(), errorRecord("boom")))

		assert.Same(t, h, h.WithAttrs(nil))
		assert.Same(t, h, h.WithGroup(""))
		assert.True(t, h.Enabled(context.Background(), slog.LevelError))
	})
}
