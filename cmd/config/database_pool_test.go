package config

import (
	"os"
	"testing"
	"time"
)

func clearPoolEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_IDLE_TIME", "DB_CONN_MAX_LIFETIME"} {
		previous, present := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(key, previous)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}
func TestDatabasePoolBudgetAndOverrides(t *testing.T) {
	clearPoolEnvironment(t)
	defaults, err := LoadDatabasePool()
	if err != nil || defaults.MaxOpen != 12 || defaults.MaxIdle != 2 || defaults.MaxIdleTime != 5*time.Minute || defaults.MaxLifetime != 30*time.Minute {
		t.Fatalf("unsafe defaults: %+v err=%v", defaults, err)
	}
	t.Setenv("DB_MAX_OPEN_CONNS", "6")
	t.Setenv("DB_MAX_IDLE_CONNS", "0")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "2m")
	t.Setenv("DB_CONN_MAX_LIFETIME", "15m")
	custom, err := LoadDatabasePool()
	if err != nil || custom.MaxOpen != 6 || custom.MaxIdle != 0 || custom.MaxIdleTime != 2*time.Minute || custom.MaxLifetime != 15*time.Minute {
		t.Fatalf("overrides ignored: %+v err=%v", custom, err)
	}
}
func TestDatabasePoolRejectsUnsafeConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"DB_MAX_OPEN_CONNS", "0"}, {"DB_MAX_OPEN_CONNS", "-1"}, {"DB_MAX_OPEN_CONNS", "invalid"}, {"DB_MAX_OPEN_CONNS", ""},
		{"DB_MAX_IDLE_CONNS", "-1"}, {"DB_MAX_IDLE_CONNS", "999"},
		{"DB_CONN_MAX_IDLE_TIME", "0s"}, {"DB_CONN_MAX_IDLE_TIME", "-1m"}, {"DB_CONN_MAX_IDLE_TIME", "invalid"},
		{"DB_CONN_MAX_LIFETIME", "0s"}, {"DB_CONN_MAX_LIFETIME", "-1m"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			clearPoolEnvironment(t)
			t.Setenv(tc.key, tc.value)
			if _, err := LoadDatabasePool(); err == nil {
				t.Fatal("unsafe pool configuration accepted")
			}
		})
	}
}
