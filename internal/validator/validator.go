package validator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/JakubPluta/tmago/internal/config"
	"github.com/JakubPluta/tmago/internal/logger"
)

// ValidationResult represents the result of validating an HTTP response.
type ValidationResult struct {
	IsValid    bool
	Errors     []string
	Duration   time.Duration
	StatusCode int
	Body       []byte
}

// Validator is a struct that validates HTTP responses based on a set of expectations.
type Validator struct {
	maxDuration time.Duration
	statusCode  int
	logger      *logger.Logger
}

// NewValidator creates a new Validator instance with specified maximum duration
// and expected HTTP status code. The Validator can be used to validate HTTP
// responses based on these criteria.
func NewValidator(maxDuration time.Duration, expectedStatus int) *Validator {
	logger, err := logger.NewLogger("logs")
	if err != nil {
		fmt.Printf("failed to create logger: %v", err)
	}
	return &Validator{
		maxDuration: maxDuration,
		statusCode:  expectedStatus,
		logger:      logger,
	}
}

// Validate validates an HTTP response against a set of expectations.
//
// The function takes an HTTP response, its body, the time it took to receive the response,
// and a list of value checks. It returns a ValidationResult with the validation result
// and any errors that occurred during the validation.
//
// The validation process is as follows:
//
//  1. The function checks if the response status code matches the expected status code.
//  2. It checks if the response time is less than the expected maximum duration.
//  3. If value checks are provided, it unmarshals the response body into a map and checks
//     if the values at the specified paths match the expected values.
func (r *Validator) Validate(resp *http.Response, body []byte, duration time.Duration, valueChecks []config.ValueCheck) ValidationResult {
	result := ValidationResult{
		Duration: duration,
		Errors:   make([]string, 0),
	}

	// validate status code
	if resp.StatusCode != r.statusCode {
		r.logger.Warn(fmt.Sprintf("expected status code %d, got %d", r.statusCode, resp.StatusCode))
		result.Errors = append(result.Errors, fmt.Sprintf("expected status code %d, got %d", r.statusCode, resp.StatusCode))
	}

	// Response time validation
	if duration > r.maxDuration {
		r.logger.Warn(fmt.Sprintf("expected response time less than %s, got %s", r.maxDuration, duration))
		result.Errors = append(result.Errors, fmt.Sprintf("expected response time less than %s, got %s", r.maxDuration, duration))
	}
	// value checks
	if len(valueChecks) > 0 {
		var responseData map[string]interface{}
		if err := json.Unmarshal(body, &responseData); err != nil {
			r.logger.Warn(fmt.Sprintf("failed to unmarshal response body: %v", err))
			result.Errors = append(result.Errors, fmt.Sprintf("failed to unmarshal response body: %v", err))
		} else {
			for _, check := range valueChecks {
				if val, ok := responseData[check.Path]; !ok {
					r.logger.Warn(fmt.Sprintf("path %s not found in response", check.Path))
					result.Errors = append(result.Errors, fmt.Sprintf("path %s not found in response", check.Path))
				} else if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", check.Value) {
					r.logger.Info(fmt.Sprintf("type of val %T and expected %T", val, check.Value))
					r.logger.Warn(fmt.Sprintf("path %s expected %v, got %v", check.Path, check.Value, val))
					result.Errors = append(result.Errors, fmt.Sprintf("path %s expected %v, got %v", check.Path, check.Value, val))
				}

			}
		}
	}
	result.IsValid = len(result.Errors) == 0
	if !result.IsValid {
		r.logger.Warn(fmt.Sprintf("validation failed: %v", result.Errors))
	}
	return result
}
