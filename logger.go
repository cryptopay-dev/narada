package narada

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/viper"
)

// ErrorKey is the attribute key narada uses to carry an error on a log record.
// The Sentry and Slack handlers look for it to report the underlying error
// instead of synthesising one from the message.
const ErrorKey = "error"

// fatalFlushTimeout bounds how long Fatal waits for Sentry to drain before exiting.
const fatalFlushTimeout = 2 * time.Second

// Err wraps err as a log attribute under ErrorKey. It is the slog replacement
// for logrus' WithError.
func Err(err error) slog.Attr {
	return slog.Any(ErrorKey, err)
}

// NewLogger builds the application logger from configuration.
//
// The handler chain is Slack -> Sentry (optional) -> text/json writer, so an
// error record reaches both sinks and is still written to stderr.
func NewLogger(config *viper.Viper) (*slog.Logger, error) {
	return newLogger(config, os.Stderr)
}

func newLogger(config *viper.Viper, out io.Writer) (*slog.Logger, error) {
	config.SetDefault("logger.formatter", "text")
	config.SetDefault("logger.level", "debug")
	config.SetDefault("logger.catch_errors", true)

	level, err := parseLevel(config.GetString("logger.level"))
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{Level: level, ReplaceAttr: lowercaseLevel}

	// Setting formatter
	var handler slog.Handler
	switch format := config.GetString("logger.formatter"); format {
	case "text":
		handler = slog.NewTextHandler(out, opts)
	case "json":
		handler = slog.NewJSONHandler(out, opts)
	default:
		return nil, fmt.Errorf("unknown formatter: %s", format)
	}

	// Catching errors with Sentry
	if config.GetBool("logger.catch_errors") {
		handler = NewSentryHandler(handler)
	}

	handler = NewSlackHandler(config, handler)

	return slog.New(handler), nil
}

// NewNopLogger returns a logger that discards every record.
func NewNopLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// Fatal logs at error level and terminates the process, replacing logrus' Fatal.
// slog has no fatal level, so the record is emitted at slog.LevelError; Sentry is
// flushed first so the report survives the exit.
func Fatal(logger *slog.Logger, msg string, args ...any) {
	logger.Error(msg, args...)
	sentry.Flush(fatalFlushTimeout)
	os.Exit(1)
}

// lowercaseLevel renders the level the way logrus did (debug/info/warning/error)
// instead of slog's upper-case names, so existing log queries and alerts keep matching.
func lowercaseLevel(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 || a.Key != slog.LevelKey {
		return a
	}

	level, ok := a.Value.Any().(slog.Level)
	if !ok {
		return a
	}

	if level == slog.LevelWarn {
		return slog.String(slog.LevelKey, "warning")
	}

	return slog.String(slog.LevelKey, strings.ToLower(level.String()))
}

func parseLevel(lvl string) (slog.Level, error) {
	switch lvl {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown logger level provided: %s", lvl)
	}
}
