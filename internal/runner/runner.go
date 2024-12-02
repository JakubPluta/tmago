package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/JakubPluta/tmago/internal/config"
	"github.com/JakubPluta/tmago/internal/logger"
	"github.com/JakubPluta/tmago/internal/reporter"
	"github.com/JakubPluta/tmago/internal/validator"
)

const (
	DefaultReportsDir = "reports"
)

// Runner is a struct that runs tests on endpoints.
type Runner struct {
	config   *config.Config
	client   *http.Client
	logger   *logger.Logger
	reporter *reporter.Reporter
}

// NewRunner returns a new Runner instance with the given config.
// The underlying HTTP client has a timeout of 30 seconds.
func NewRunner(cfg *config.Config) (*Runner, error) {
	logger, err := logger.NewLogger("logs")
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &Runner{
		config:   cfg,
		client:   &http.Client{Timeout: time.Second * 30},
		logger:   logger,
		reporter: reporter.NewReporter(),
	}, nil
}

// Run runs all the tests in the given config concurrently.
// It will call either runSingle or runConcurrent for each endpoint,
// depending on whether the endpoint has concurrency configuration.
// The function will return an error if any of the calls to runSingle
// or runConcurrent return an error.
func (r *Runner) Run(ctx context.Context) error {
	for _, endpoint := range r.config.Endpoints {
		r.logger.TestStarted(endpoint.Name, endpoint.Method, endpoint.URL)

		result := reporter.TestResult{
			EndpointName: endpoint.Name,
			Method:       endpoint.Method,
			URL:          endpoint.URL,
			StartTime:    time.Now(),
			StatusCodes:  make(map[int]int),
		}

		if endpoint.Concurrent.Users > 0 {
			err := r.runConcurrent(ctx, endpoint, &result)
			if err != nil {
				r.logger.RequestFailed(-1, endpoint.Name, err)
				result.Errors = append(result.Errors, err.Error())
			}
		} else {
			err := r.runSingle(ctx, endpoint, &result)
			if err != nil {
				r.logger.RequestFailed(-1, endpoint.Name, err)
				result.Errors = append(result.Errors, err.Error())
			}
		}

		result.EndTime = time.Now()
		r.reporter.AddResult(result)
		r.logger.Info(fmt.Sprintf("Test %s completed. With parameters: TotalRequests: %d, ConcurrentUsers: %d, ConcurrentRequests: %d", endpoint.Name, endpoint.Concurrent.Total, endpoint.Concurrent.Users, endpoint.Concurrent.Total/endpoint.Concurrent.Users))
	}

	// Generate report
	if err := r.reporter.GenerateHTML("report.html"); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	return nil
}
func (r *Runner) runSingle(ctx context.Context, endpoint config.Endpoint, result *reporter.TestResult) error {
	var lastErr error
	for i := 0; i <= endpoint.Retry.Count; i++ {
		if i > 0 {
			time.Sleep(endpoint.Retry.Delay)
		}

		start := time.Now()
		resp, err := r.makeRequest(ctx, endpoint)
		duration := time.Since(start)

		if err != nil {
			lastErr = err
			continue
		}

		result.TotalRequests++
		result.StatusCodes[resp.StatusCode]++

		if resp.IsValid {
			result.SuccessCount++
			result.AverageLatency = duration
			return nil
		}

		lastErr = fmt.Errorf("validation failed: %v", resp.Errors)
	}

	result.FailureCount++
	return lastErr
}

func (r *Runner) runConcurrent(ctx context.Context, endpoint config.Endpoint, result *reporter.TestResult) error {
	var wg sync.WaitGroup
	resultChan := make(chan validator.ValidationResult, endpoint.Concurrent.Total)
	errChan := make(chan error, endpoint.Concurrent.Total)

	requestsPerUser := endpoint.Concurrent.Total / endpoint.Concurrent.Users

	result.IsConcurrent = true
	result.ConcurrentUsers = endpoint.Concurrent.Users
	result.TotalRequests = endpoint.Concurrent.Total

	// Start goroutines for each user
	for i := 0; i < endpoint.Concurrent.Users; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			for j := 0; j < requestsPerUser; j++ {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				default:
					resp, err := r.makeRequest(ctx, endpoint)
					if err != nil {
						errChan <- err
						return
					}
					resultChan <- resp
					if endpoint.Concurrent.Delay > 0 {
						time.Sleep(endpoint.Concurrent.Delay)
					}
				}
			}
		}(i)
	}

	// Close channels when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
		close(errChan)
	}()

	// Process results and collect stats
	var totalLatency time.Duration
	for resp := range resultChan {
		result.StatusCodes[resp.StatusCode]++
		if resp.IsValid {
			result.SuccessCount++
		} else {
			result.FailureCount++
			result.Errors = append(result.Errors, resp.Errors...)
		}

		if resp.Duration < result.MinLatency || result.MinLatency == 0 {
			result.MinLatency = resp.Duration
		}
		if resp.Duration > result.MaxLatency {
			result.MaxLatency = resp.Duration
		}
		totalLatency += resp.Duration
	}

	// Check for errors
	var lastErr error
	for err := range errChan {
		if err != nil {
			if lastErr == nil {
				lastErr = err
			} else {
				lastErr = fmt.Errorf("%v; %v", lastErr, err)
			}
		}
	}

	if result.TotalRequests > 0 {
		result.AverageLatency = totalLatency / time.Duration(result.SuccessCount+result.FailureCount)
	}

	return lastErr
}

// makeRequest makes an HTTP request to the given endpoint and validates
// the response with the given expectation. The request is made with the
// context given as the first argument.
//
// The function returns a ValidationResult with the validation result and
// any error that occurred during the request or validation.
func (r *Runner) makeRequest(ctx context.Context, endpoint config.Endpoint) (validator.ValidationResult, error) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, endpoint.Method, endpoint.URL, bytes.NewBufferString(endpoint.Body))
	if err != nil {
		return validator.ValidationResult{}, err
	}

	// headers
	for k, v := range endpoint.Headers {
		req.Header.Add(k, v)
	}

	// Make request
	resp, err := r.client.Do(req)
	if err != nil {
		return validator.ValidationResult{}, err
	}
	defer resp.Body.Close() // Close the response body when the function returns

	duration := time.Since(start)

	// read body

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return validator.ValidationResult{}, err
	}

	// validate response
	v := validator.NewValidator(endpoint.Expect.MaxTime, endpoint.Expect.Status)
	return v.Validate(resp, body, duration, endpoint.Expect.Values), nil

}
