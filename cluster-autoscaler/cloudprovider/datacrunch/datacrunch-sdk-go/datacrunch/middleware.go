package datacrunch

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// RequestContext holds the context for request middleware
type RequestContext struct {
	Method  string
	Path    string
	Body    interface{}
	Headers http.Header
	Query   url.Values
	Request *http.Request
	Client  *Client
}

// ResponseContext holds the context for response middleware
type ResponseContext struct {
	Request    *RequestContext
	Response   *http.Response
	Body       []byte
	Error      error
	StatusCode int
}

// RequestMiddleware defines the request middleware function signature
type RequestMiddleware func(next RequestHandler) RequestHandler

// ResponseMiddleware defines the response middleware function signature
type ResponseMiddleware func(next ResponseHandler) ResponseHandler

// RequestHandler processes the request
type RequestHandler func(ctx *RequestContext) error

// ResponseHandler processes the response
type ResponseHandler func(ctx *ResponseContext) error

// AuthenticationMiddleware adds authentication headers to requests
func AuthenticationMiddleware() RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			// Get bearer token
			bearerToken, err := ctx.Client.Auth.GetBearerToken()
			if err != nil {
				return fmt.Errorf("failed to get authentication token: %w", err)
			}

			if bearerToken == "" {
				return fmt.Errorf("empty authentication token")
			}

			// Add authorization header
			ctx.Headers.Set("Authorization", bearerToken)

			return next(ctx)
		}
	}
}

// ContentTypeMiddleware sets the Content-Type header for requests with body
func ContentTypeMiddleware(contentType string) RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			if ctx.Body != nil {
				ctx.Headers.Set("Content-Type", contentType)
			}
			return next(ctx)
		}
	}
}

// JSONContentTypeMiddleware is a convenience middleware for JSON content type
func JSONContentTypeMiddleware() RequestMiddleware {
	return ContentTypeMiddleware("application/json")
}

// LoggingMiddleware logs request details using the client's logger
func LoggingMiddleware(logger Logger) RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			start := time.Now()
			logger.Debug("Starting %s request to %s", ctx.Method, ctx.Path)

			err := next(ctx)

			duration := time.Since(start)
			if err != nil {
				logger.Debug("Request %s %s failed after %v: %v", ctx.Method, ctx.Path, duration, err)
			} else {
				logger.Debug("Request %s %s completed in %v", ctx.Method, ctx.Path, duration)
			}

			return err
		}
	}
}

// UserAgentMiddleware adds a User-Agent header
func UserAgentMiddleware(userAgent string) RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			ctx.Headers.Set("User-Agent", userAgent)
			return next(ctx)
		}
	}
}

// RetryMiddleware implements retry logic for failed requests
func RetryMiddleware(maxRetries int, retryDelay time.Duration, logger Logger) RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					logger.Debug("Retrying request %s %s (attempt %d/%d)", ctx.Method, ctx.Path, attempt+1, maxRetries+1)
					time.Sleep(retryDelay)
				}

				lastErr = next(ctx)
				if lastErr == nil {
					return nil // Success
				}

				// Don't retry on certain errors (like authentication failures)
				if !shouldRetry(lastErr) {
					break
				}
			}

			return fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
		}
	}
}

// shouldRetry determines if an error is retryable
func shouldRetry(err error) bool {
	// Add logic to determine if error is retryable
	// For now, we'll retry on most errors except authentication
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Don't retry authentication errors
	if contains(errStr, "authentication") || contains(errStr, "unauthorized") {
		return false
	}

	return true
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsHelper(s, substr))))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Response middleware implementations

// ErrorHandlingMiddleware handles HTTP error responses
func ErrorHandlingMiddleware() ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			// Check for HTTP errors
			if ctx.StatusCode < 200 || ctx.StatusCode >= 300 {
				// Try to parse as API error
				var apiError APIError
				if len(ctx.Body) > 0 {
					// This would need proper JSON parsing
					apiError = APIError{
						StatusCode: ctx.StatusCode,
						Message:    string(ctx.Body),
					}
				} else {
					apiError = APIError{
						StatusCode: ctx.StatusCode,
						Message:    http.StatusText(ctx.StatusCode),
					}
				}
				ctx.Error = &apiError
			}

			return next(ctx)
		}
	}
}

// ResponseLoggingMiddleware logs response details using the client's logger
func ResponseLoggingMiddleware(logger Logger) ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			logger.Debug("Response for %s %s: Status %d, Body length: %d bytes",
				ctx.Request.Method, ctx.Request.Path, ctx.StatusCode, len(ctx.Body))

			if ctx.Error != nil {
				logger.Debug("Response error: %v", ctx.Error)
			}

			return next(ctx)
		}
	}
}

// MetricsMiddleware could collect metrics about requests/responses
func MetricsMiddleware(logger Logger) ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			// Here you could collect metrics like:
			// - Response time
			// - Status codes
			// - Error rates
			// - Request/response sizes

			// For now, just log basic metrics
			logger.Debug("Metrics: %s %s -> %d (%d bytes)",
				ctx.Request.Method, ctx.Request.Path, ctx.StatusCode, len(ctx.Body))

			return next(ctx)
		}
	}
}

// CacheMiddleware could implement response caching
func CacheMiddleware() ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			// Cache implementation would go here
			// For now, just pass through
			return next(ctx)
		}
	}
}
