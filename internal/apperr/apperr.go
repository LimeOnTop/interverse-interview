package apperr

import (
	"log"
	"strings"
)

// DevMode controls whether API/proto responses include internal error details.
// Default false. Set via Configure from config DEV_MODE.
var DevMode bool

func Configure(devMode bool) {
	DevMode = devMode
}

// Message returns err.Error() in DevMode.
// When DevMode is off, business-looking messages are kept; infra details
// are replaced with the provided public fallback. The real error is always logged.
func Message(err error, public string) string {
	if err == nil {
		return public
	}
	log.Printf("%s: %v", public, err)
	if DevMode {
		return err.Error()
	}
	raw := strings.TrimSpace(err.Error())
	if raw != "" && !looksInternal(raw) {
		return raw
	}
	return public
}

// Logf writes a log line only when DevMode is enabled.
func Logf(format string, args ...any) {
	if DevMode {
		log.Printf(format, args...)
	}
}

func looksInternal(msg string) bool {
	lower := strings.ToLower(msg)
	markers := []string{
		"pq:",
		"sql:",
		"redis",
		"rpc error",
		"connection refused",
		"dial tcp",
		"i/o timeout",
		"context deadline",
		"context canceled",
		"driver:",
		"kafka",
		"elasticsearch",
		"begin tx",
		"commit tx",
		"wrap:",
		"panic:",
		"stack",
		"goroutine",
		"/users/",
		"/home/",
		"password=",
		"postgres://",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
