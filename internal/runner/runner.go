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

type Runner struct {
	config   *config.Config
	client   *http.Client
	logger   *logger.Logger
	reporter *reporter.Reporter
}

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

func (r *Runner) Run(ctx context.Context) error {
	r.reporter.StartTest() // Initialize start time

	for _, endpoint := range r.config.Endpoints {
		r.logger.TestStarted(endpoint.Name, endpoint.Method, endpoint.URL)

		result := reporter.TestResult{
			EndpointName:       endpoint.Name,
			Method:             endpoint.Method,
			URL:                endpoint.URL,
			StartTime:          time.Now(),
			StatusCodes:        make(map[int]int),
			ValidationFailures: make(map[string]int),
			RequestDetails:     make([]reporter.RequestDetail, 0),
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
		duration := result.EndTime.Sub(result.StartTime)
		result.RequestsPerSecond = float64(result.TotalRequests) / duration.Seconds()
		result.ErrorRate = float64(result.FailureCount) / float64(result.TotalRequests) * 100

		r.reporter.AddResult(result)
		r.logger.Info(fmt.Sprintf("Test %s completed. TotalRequests: %d, Success: %d, Failures: %d",
			endpoint.Name, result.TotalRequests, result.SuccessCount, result.FailureCount))
	}

	return r.reporter.GenerateHTML("reports/report.html")
}

func (r *Runner) runSingle(ctx context.Context, endpoint config.Endpoint, result *reporter.TestResult) error {
	var lastErr error

	for i := 0; i <= endpoint.Retry.Count; i++ {
		if i > 0 {
			time.Sleep(endpoint.Retry.Delay)
		}

		requestDetail := reporter.RequestDetail{
			ID:        result.TotalRequests + 1,
			Timestamp: time.Now(),
		}

		resp, body, duration, err := r.makeRequest(ctx, endpoint)
		requestDetail.Duration = duration

		if err != nil {
			lastErr = err
			requestDetail.Success = false
			requestDetail.ErrorMessage = err.Error()
			result.RequestDetails = append(result.RequestDetails, requestDetail)
			continue
		}

		requestDetail.StatusCode = resp.StatusCode
		requestDetail.ResponseSize = int64(len(body))
		requestDetail.Headers = make(map[string]string)
		for k, v := range resp.Header {
			requestDetail.Headers[k] = v[0]
		}

		validationResult := r.validateResponse(resp, body, duration, endpoint)
		requestDetail.Success = validationResult.IsValid
		requestDetail.ValidationErrors = validationResult.Errors

		result.TotalRequests++
		result.StatusCodes[resp.StatusCode]++
		result.BytesTransferred += int64(len(body))

		if validationResult.IsValid {
			result.SuccessCount++
			if result.MinLatency == 0 || duration < result.MinLatency {
				result.MinLatency = duration
			}
			if duration > result.MaxLatency {
				result.MaxLatency = duration
			}
		} else {
			result.FailureCount++
			for _, err := range validationResult.Errors {
				result.ValidationFailures[err]++
			}
		}

		result.RequestDetails = append(result.RequestDetails, requestDetail)

		if validationResult.IsValid {
			return nil
		}

		lastErr = fmt.Errorf("validation failed: %v", validationResult.Errors)
	}

	return lastErr
}

func (r *Runner) runConcurrent(ctx context.Context, endpoint config.Endpoint, result *reporter.TestResult) error {
	var wg sync.WaitGroup
	requestChan := make(chan reporter.RequestDetail, endpoint.Concurrent.Total)
	errChan := make(chan error, endpoint.Concurrent.Total)

	requestsPerUser := endpoint.Concurrent.Total / endpoint.Concurrent.Users
	result.IsConcurrent = true
	result.ConcurrentUsers = endpoint.Concurrent.Users

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
					requestID := userID*requestsPerUser + j + 1
					detail := reporter.RequestDetail{
						ID:        requestID,
						Timestamp: time.Now(),
					}

					resp, body, duration, err := r.makeRequest(ctx, endpoint)
					detail.Duration = duration

					if err != nil {
						detail.Success = false
						detail.ErrorMessage = err.Error()
						requestChan <- detail
						errChan <- err
						continue
					}

					detail.StatusCode = resp.StatusCode
					detail.ResponseSize = int64(len(body))
					detail.Headers = make(map[string]string)
					for k, v := range resp.Header {
						detail.Headers[k] = v[0]
					}

					validationResult := r.validateResponse(resp, body, duration, endpoint)
					detail.Success = validationResult.IsValid
					detail.ValidationErrors = validationResult.Errors

					requestChan <- detail

					if endpoint.Concurrent.Delay > 0 {
						time.Sleep(endpoint.Concurrent.Delay)
					}
				}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(requestChan)
		close(errChan)
	}()

	var totalLatency time.Duration
	var minLatency time.Duration
	var maxLatency time.Duration
	var totalBytes int64

	for detail := range requestChan {
		result.RequestDetails = append(result.RequestDetails, detail)
		result.TotalRequests++
		result.StatusCodes[detail.StatusCode]++
		result.BytesTransferred += detail.ResponseSize

		if detail.Success {
			result.SuccessCount++
			if minLatency == 0 || detail.Duration < minLatency {
				minLatency = detail.Duration
			}
			if detail.Duration > maxLatency {
				maxLatency = detail.Duration
			}
			totalLatency += detail.Duration
			totalBytes += detail.ResponseSize
		} else {
			result.FailureCount++
			for _, err := range detail.ValidationErrors {
				result.ValidationFailures[err]++
			}
		}
	}

	if result.SuccessCount > 0 {
		result.MinLatency = minLatency
		result.MaxLatency = maxLatency
		result.AverageLatency = totalLatency / time.Duration(result.SuccessCount)
		result.ResponseSizes.Min = totalBytes / int64(result.SuccessCount)
		result.ResponseSizes.Max = totalBytes / int64(result.SuccessCount)
		result.ResponseSizes.Avg = totalBytes / int64(result.SuccessCount)
	}

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

	return lastErr
}

func (r *Runner) makeRequest(ctx context.Context, endpoint config.Endpoint) (*http.Response, []byte, time.Duration, error) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, endpoint.Method, endpoint.URL, bytes.NewBufferString(endpoint.Body))
	if err != nil {
		return nil, nil, 0, err
	}

	for k, v := range endpoint.Headers {
		req.Header.Add(k, v)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, nil, time.Since(start), err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, time.Since(start), err
	}

	return resp, body, time.Since(start), nil
}

func (r *Runner) validateResponse(resp *http.Response, body []byte, duration time.Duration, endpoint config.Endpoint) validator.ValidationResult {
	v := validator.NewValidator(endpoint.Expect.MaxTime, endpoint.Expect.Status)
	return v.Validate(resp, body, duration, endpoint.Expect.Values)
}
