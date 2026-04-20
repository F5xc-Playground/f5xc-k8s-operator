package xcclient_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentinelErrorsExist(t *testing.T) {
	assert.NotNil(t, xcclient.ErrNotFound)
	assert.NotNil(t, xcclient.ErrConflict)
	assert.NotNil(t, xcclient.ErrRateLimited)
	assert.NotNil(t, xcclient.ErrServerError)
	assert.NotNil(t, xcclient.ErrAuth)
}

func TestAPIErrorWrapssentinel(t *testing.T) {
	tests := []struct {
		name     string
		sentinel error
	}{
		{"ErrNotFound", xcclient.ErrNotFound},
		{"ErrConflict", xcclient.ErrConflict},
		{"ErrRateLimited", xcclient.ErrRateLimited},
		{"ErrServerError", xcclient.ErrServerError},
		{"ErrAuth", xcclient.ErrAuth},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			apiErr := &xcclient.APIError{
				StatusCode: 999,
				Endpoint:   "/test",
				Message:    "test message",
				Err:        tc.sentinel,
			}
			assert.True(t, errors.Is(apiErr, tc.sentinel),
				"errors.Is should find sentinel through APIError")
		})
	}
}

func TestAPIErrorUnwrap(t *testing.T) {
	apiErr := &xcclient.APIError{
		StatusCode: 404,
		Endpoint:   "/api/v1/resource",
		Message:    "not found",
		Err:        xcclient.ErrNotFound,
	}

	var target *xcclient.APIError
	require.True(t, errors.As(apiErr, &target), "errors.As should extract *APIError")
	assert.Equal(t, 404, target.StatusCode)
	assert.Equal(t, "/api/v1/resource", target.Endpoint)
	assert.Equal(t, "not found", target.Message)
	assert.Equal(t, xcclient.ErrNotFound, target.Err)
}

func TestAPIErrorMessage(t *testing.T) {
	apiErr := &xcclient.APIError{
		StatusCode: 404,
		Endpoint:   "/api/v1/resource",
		Message:    "object not found",
		Err:        xcclient.ErrNotFound,
	}

	msg := apiErr.Error()
	assert.Contains(t, msg, "404")
	assert.Contains(t, msg, "/api/v1/resource")
	assert.Contains(t, msg, "object not found")
}

func TestStatusToError(t *testing.T) {
	tests := []struct {
		statusCode int
		sentinel   error
	}{
		{404, xcclient.ErrNotFound},
		{401, xcclient.ErrAuth},
		{403, xcclient.ErrAuth},
		{409, xcclient.ErrConflict},
		{429, xcclient.ErrRateLimited},
		{500, xcclient.ErrServerError},
		{502, xcclient.ErrServerError},
		{503, xcclient.ErrServerError},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("status_%d", tc.statusCode), func(t *testing.T) {
			err := xcclient.StatusToError(tc.statusCode, "/some/endpoint", "body text")
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.sentinel),
				"StatusToError(%d) should wrap %v", tc.statusCode, tc.sentinel)

			var apiErr *xcclient.APIError
			require.True(t, errors.As(err, &apiErr),
				"StatusToError(%d) should return an *APIError", tc.statusCode)
			assert.Equal(t, tc.statusCode, apiErr.StatusCode)
			assert.Equal(t, "/some/endpoint", apiErr.Endpoint)
			assert.Equal(t, "body text", apiErr.Message)
		})
	}
}

func TestStatusToErrorOther(t *testing.T) {
	// A status code that doesn't match any sentinel should still return an error.
	err := xcclient.StatusToError(400, "/bad/request", "bad request body")
	require.Error(t, err)
	// Should NOT match any of the typed sentinels.
	assert.False(t, errors.Is(err, xcclient.ErrNotFound))
	assert.False(t, errors.Is(err, xcclient.ErrAuth))
	assert.False(t, errors.Is(err, xcclient.ErrConflict))
	assert.False(t, errors.Is(err, xcclient.ErrRateLimited))
	assert.False(t, errors.Is(err, xcclient.ErrServerError))
}
