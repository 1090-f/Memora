package middleware

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelectRequestLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		status   int
		duration time.Duration
		want     requestLogLevel
	}{
		{name: "successful get", method: http.MethodGet, status: http.StatusOK, duration: 10 * time.Millisecond, want: requestLogDebug},
		{name: "successful head", method: http.MethodHead, status: http.StatusOK, duration: 10 * time.Millisecond, want: requestLogDebug},
		{name: "successful options", method: http.MethodOptions, status: http.StatusNoContent, duration: 10 * time.Millisecond, want: requestLogDebug},
		{name: "successful mutation", method: http.MethodPost, status: http.StatusCreated, duration: 10 * time.Millisecond, want: requestLogInfo},
		{name: "client error", method: http.MethodGet, status: http.StatusNotFound, duration: 10 * time.Millisecond, want: requestLogWarn},
		{name: "server error", method: http.MethodGet, status: http.StatusInternalServerError, duration: 10 * time.Millisecond, want: requestLogWarn},
		{name: "slow read", method: http.MethodGet, status: http.StatusOK, duration: slowRequestThreshold, want: requestLogWarn},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, selectRequestLogLevel(test.method, test.status, test.duration))
		})
	}
}
