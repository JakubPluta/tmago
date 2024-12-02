// internal/reporter/reporter.go
package reporter

import (
	"fmt"
	"html/template"
	"os"
	"time"
)

// TestResult represents the result of a single test
type TestResult struct {
	EndpointName    string
	Method          string
	URL             string
	StartTime       time.Time
	EndTime         time.Time
	TotalRequests   int
	SuccessCount    int
	FailureCount    int
	AverageLatency  time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	StatusCodes     map[int]int
	Errors          []string
	IsConcurrent    bool
	ConcurrentUsers int
}

// Report represents the final report
type Report struct {
	TestResults    []TestResult
	StartTime      time.Time
	EndTime        time.Time
	TotalEndpoints int
	SuccessRate    float64
	ChartData      ChartData
}

type ChartData struct {
	Labels []string
	Values []float64
}

type Reporter struct {
	results []TestResult
	start   time.Time
}

// NewReporter creates and returns a new Reporter instance.
// The Reporter is initialized with an empty slice of TestResult
// and sets the start time to the current time.
func NewReporter() *Reporter {
	return &Reporter{
		results: make([]TestResult, 0),
		start:   time.Now(),
	}
}

// AddResult adds the given TestResult to the Reporter's internal
// results. It should be called after each test has been run.
func (r *Reporter) AddResult(result TestResult) {
	r.results = append(r.results, result)
}

func (r *Reporter) GenerateHTML(outputPath string) error {
	report := r.prepareReport()

	// Read template
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create output file
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	// Execute template
	if err := tmpl.Execute(f, report); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (r *Reporter) prepareReport() Report {
	var totalSuccess, totalRequests int
	for _, result := range r.results {
		totalSuccess += result.SuccessCount
		totalRequests += result.TotalRequests
	}

	successRate := float64(totalSuccess) / float64(totalRequests) * 100

	return Report{
		TestResults:    r.results,
		StartTime:      r.start,
		EndTime:        time.Now(),
		TotalEndpoints: len(r.results),
		SuccessRate:    successRate,
		ChartData:      r.prepareChartData(),
	}
}

func (r *Reporter) prepareChartData() ChartData {
	var labels []string
	var values []float64
	for _, result := range r.results {
		labels = append(labels, result.EndpointName)
		values = append(values, float64(result.SuccessCount)/float64(result.TotalRequests)*100)
	}
	return ChartData{Labels: labels, Values: values}
}

const reportTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>API Test Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <link href="https://cdn.jsdelivr.net/npm/tailwindcss@2.2.19/dist/tailwind.min.css" rel="stylesheet">
</head>
<body class="bg-gray-100 p-8">
    <div class="max-w-7xl mx-auto">
        <div class="bg-white rounded-lg shadow-lg p-6 mb-8">
            <h1 class="text-3xl font-bold mb-4">API Test Report</h1>
            
            <!-- Summary -->
            <div class="grid grid-cols-4 gap-4 mb-8">
                <div class="bg-blue-50 p-4 rounded-lg">
                    <h3 class="text-lg font-semibold text-blue-700">Total Endpoints</h3>
                    <p class="text-2xl">{{.TotalEndpoints}}</p>
                </div>
                <div class="bg-green-50 p-4 rounded-lg">
                    <h3 class="text-lg font-semibold text-green-700">Success Rate</h3>
                    <p class="text-2xl">{{printf "%.2f" .SuccessRate}}%</p>
                </div>
                <div class="bg-purple-50 p-4 rounded-lg">
                    <h3 class="text-lg font-semibold text-purple-700">Duration</h3>
                    <p class="text-2xl">{{.EndTime.Sub .StartTime}}</p>
                </div>
            </div>

            <!-- Charts -->
            <div class="mb-8">
                <canvas id="latencyChart"></canvas>
            </div>

            <!-- Detailed Results -->
            <div class="mt-8">
                <h2 class="text-2xl font-bold mb-4">Detailed Results</h2>
                {{range .TestResults}}
                <div class="bg-gray-50 p-4 rounded-lg mb-4">
                    <div class="flex justify-between items-center mb-2">
                        <h3 class="text-xl font-semibold">{{.EndpointName}}</h3>
                        <span class="px-3 py-1 rounded-full {{if gt .SuccessCount .FailureCount}}bg-green-100 text-green-800{{else}}bg-red-100 text-red-800{{end}}">
                            {{.SuccessCount}}/{{.TotalRequests}} Success
                        </span>
                    </div>
                    <div class="grid grid-cols-2 gap-4 text-sm">
                        <div>
                            <p><strong>Method:</strong> {{.Method}}</p>
                            <p><strong>URL:</strong> {{.URL}}</p>
                            <p><strong>Average Latency:</strong> {{.AverageLatency}}</p>
                        </div>
                        <div>
                            <p><strong>Min Latency:</strong> {{.MinLatency}}</p>
                            <p><strong>Max Latency:</strong> {{.MaxLatency}}</p>
                            {{if .IsConcurrent}}
                            <p><strong>Concurrent Users:</strong> {{.ConcurrentUsers}}</p>
                            {{end}}
                        </div>
                    </div>
                    {{if .Errors}}
                    <div class="mt-2">
                        <h4 class="font-semibold text-red-600">Errors:</h4>
                        <ul class="list-disc list-inside">
                            {{range .Errors}}
                            <li class="text-red-600">{{.}}</li>
                            {{end}}
                        </ul>
                    </div>
                    {{end}}
                </div>
                {{end}}
            </div>
        </div>
    </div>

    <script>
    const ctx = document.getElementById('latencyChart').getContext('2d');
    new Chart(ctx, {
        type: 'bar',
        data: {
            labels: {{.ChartData.Labels}},
            datasets: [{
                label: 'Average Latency (ms)',
                data: {{.ChartData.Values}},
                backgroundColor: 'rgba(54, 162, 235, 0.2)',
                borderColor: 'rgba(54, 162, 235, 1)',
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            scales: {
                y: {
                    beginAtZero: true
                }
            }
        }
    });
    </script>
</body>
</html>
`
