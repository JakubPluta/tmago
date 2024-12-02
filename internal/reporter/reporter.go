// internal/reporter/reporter.go
package reporter

import (
	"fmt"
	"html/template"
	"os"
	"sort"
	"time"
)

type Reporter struct {
	results []TestResult
	start   time.Time
}

func NewReporter() *Reporter {
	return &Reporter{
		results: make([]TestResult, 0),
	}
}

func (r *Reporter) StartTest() {
	r.start = time.Now()
}

func (r *Reporter) AddResult(result TestResult) {
	// Calculate additional metrics before adding the result
	durations := make([]time.Duration, 0, len(result.RequestDetails))
	for _, detail := range result.RequestDetails {
		durations = append(durations, detail.Duration)
	}

	// Calculate percentiles
	result.Percentiles = calculatePercentiles(durations)

	// Calculate response size statistics
	if len(result.RequestDetails) > 0 {
		var minSize, maxSize, totalSize int64
		minSize = result.RequestDetails[0].ResponseSize
		for _, detail := range result.RequestDetails {
			size := detail.ResponseSize
			if size < minSize {
				minSize = size
			}
			if size > maxSize {
				maxSize = size
			}
			totalSize += size
		}
		result.ResponseSizes.Min = minSize
		result.ResponseSizes.Max = maxSize
		result.ResponseSizes.Avg = totalSize / int64(len(result.RequestDetails))
	}

	r.results = append(r.results, result)
}

type RequestDetail struct {
	ID               int
	Timestamp        time.Time
	Duration         time.Duration
	StatusCode       int
	Success          bool
	ErrorMessage     string
	ResponseSize     int64
	Headers          map[string]string
	ValidationErrors []string
}

type LatencyPercentiles struct {
	P50 time.Duration
	P75 time.Duration
	P90 time.Duration
	P95 time.Duration
	P99 time.Duration
}

type TestResult struct {
	EndpointName     string
	Method           string
	URL              string
	StartTime        time.Time
	EndTime          time.Time
	TotalRequests    int
	SuccessCount     int
	FailureCount     int
	AverageLatency   time.Duration
	MinLatency       time.Duration
	MaxLatency       time.Duration
	Percentiles      LatencyPercentiles
	StatusCodes      map[int]int
	Errors           []string
	IsConcurrent     bool
	ConcurrentUsers  int
	RequestDetails   []RequestDetail
	BytesTransferred int64
	ResponseSizes    struct {
		Min int64
		Max int64
		Avg int64
	}
	RequestsPerSecond  float64
	ErrorRate          float64
	TimeoutCount       int
	ValidationFailures map[string]int
}

type Report struct {
	TestResults    []TestResult
	StartTime      time.Time
	EndTime        time.Time
	TotalEndpoints int
	SuccessRate    float64
	TotalRequests  int
	GlobalStats    struct {
		AverageLatency    time.Duration
		MaxLatency        time.Duration
		MinLatency        time.Duration
		TotalErrors       int
		TotalTimeouts     int
		TotalBytes        int64
		RequestsPerSecond float64
	}
	ChartData ChartData
}

type ChartData struct {
	Labels        []string
	LatencyValues []float64
	SuccessRates  []float64
	ErrorRates    []float64
	RPSValues     []float64
}

func calculatePercentiles(durations []time.Duration) LatencyPercentiles {
	if len(durations) == 0 {
		return LatencyPercentiles{}
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	return LatencyPercentiles{
		P50: durations[int(float64(len(durations))*0.50)],
		P75: durations[int(float64(len(durations))*0.75)],
		P90: durations[int(float64(len(durations))*0.90)],
		P95: durations[int(float64(len(durations))*0.95)],
		P99: durations[int(float64(len(durations))*0.99)],
	}
}

func (r *Reporter) prepareChartData() ChartData {
	data := ChartData{
		Labels:        make([]string, len(r.results)),
		LatencyValues: make([]float64, len(r.results)),
		SuccessRates:  make([]float64, len(r.results)),
		ErrorRates:    make([]float64, len(r.results)),
		RPSValues:     make([]float64, len(r.results)),
	}

	for i, result := range r.results {
		data.Labels[i] = result.EndpointName
		data.LatencyValues[i] = float64(result.AverageLatency.Milliseconds())
		data.SuccessRates[i] = float64(result.SuccessCount) / float64(result.TotalRequests) * 100
		data.ErrorRates[i] = float64(result.FailureCount) / float64(result.TotalRequests) * 100
		data.RPSValues[i] = result.RequestsPerSecond
	}

	return data
}

func (r *Reporter) prepareReport() Report {
	report := Report{
		TestResults:    r.results,
		StartTime:      r.start,
		EndTime:        time.Now(),
		TotalEndpoints: len(r.results),
	}

	var totalSuccessful, totalRequests int
	var totalLatency time.Duration
	var maxLatency time.Duration
	var minLatency = time.Hour * 24
	var totalBytes int64
	var totalErrors int
	var totalTimeouts int

	for _, result := range r.results {
		totalSuccessful += result.SuccessCount
		totalRequests += result.TotalRequests
		totalLatency += result.AverageLatency * time.Duration(result.TotalRequests)
		totalBytes += result.BytesTransferred
		totalErrors += result.FailureCount
		totalTimeouts += result.TimeoutCount

		if result.MaxLatency > maxLatency {
			maxLatency = result.MaxLatency
		}
		if result.MinLatency < minLatency {
			minLatency = result.MinLatency
		}
	}

	report.TotalRequests = totalRequests
	report.SuccessRate = float64(totalSuccessful) / float64(totalRequests) * 100

	report.GlobalStats = struct {
		AverageLatency    time.Duration
		MaxLatency        time.Duration
		MinLatency        time.Duration
		TotalErrors       int
		TotalTimeouts     int
		TotalBytes        int64
		RequestsPerSecond float64
	}{
		AverageLatency:    totalLatency / time.Duration(totalRequests),
		MaxLatency:        maxLatency,
		MinLatency:        minLatency,
		TotalErrors:       totalErrors,
		TotalTimeouts:     totalTimeouts,
		TotalBytes:        totalBytes,
		RequestsPerSecond: float64(totalRequests) / report.EndTime.Sub(report.StartTime).Seconds(),
	}

	report.ChartData = r.prepareChartData()
	return report
}

func (r *Reporter) GenerateHTML(filename string) error {
	report := r.prepareReport()

	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, report); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

const reportTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Detailed API Test Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/moment"></script>
    <link href="https://cdn.jsdelivr.net/npm/tailwindcss@2.2.19/dist/tailwind.min.css" rel="stylesheet">
</head>
<body class="bg-gray-100 p-8">
    <div class="max-w-7xl mx-auto">
        <div class="bg-white rounded-lg shadow-lg p-6 mb-8">
            <h1 class="text-3xl font-bold mb-4">API Test Report</h1>
            
            <!-- Global Summary -->
            <div class="grid grid-cols-5 gap-4 mb-8">
                <div class="bg-blue-50 p-4 rounded-lg">
                    <h3 class="text-lg font-semibold text-blue-700">Total Requests</h3>
                    <p class="text-2xl">{{.TotalRequests}}</p>
                </div>
                <div class="bg-green-50 p-4 rounded-lg">
                    <h3 class="text-lg font-semibold text-green-700">Success Rate</h3>
                    <p class="text-2xl">{{printf "%.2f" .SuccessRate}}%</p>
                </div>
                <div class="bg-purple-50 p-4 rounded-lg">
                    <h3 class="text-lg font-semibold text-purple-700">Avg Latency</h3>
                    <p class="text-2xl">{{.GlobalStats.AverageLatency}}</p>
                </div>
                <div class="bg-yellow-50 p-4 rounded-lg">
                    <h3 class="text-lg font-semibold text-yellow-700">RPS</h3>
                    <p class="text-2xl">{{printf "%.2f" .GlobalStats.RequestsPerSecond}}</p>
                </div>
                <div class="bg-indigo-50 p-4 rounded-lg">
                    <h3 class="text-lg font-semibold text-indigo-700">Concurrency</h3>
                    {{range .TestResults}}
                        {{if .IsConcurrent}}
                        <p class="text-2xl">{{.ConcurrentUsers}} users</p>
                        {{end}}
                    {{end}}
                </div>
            </div>

            <!-- Performance Charts -->
            <div class="grid grid-cols-2 gap-4 mb-8">
                <div>
                    <canvas id="latencyChart"></canvas>
                </div>
                <div>
                    <canvas id="successRateChart"></canvas>
                </div>
            </div>

            <!-- Detailed Results -->
            {{range .TestResults}}
            <div class="bg-gray-50 p-6 rounded-lg mb-6">
                <div class="flex justify-between items-center mb-4">
                    <h3 class="text-xl font-bold">{{.EndpointName}}</h3>
                    <div class="flex space-x-2">
                        <span class="px-3 py-1 rounded-full {{if gt .SuccessCount .FailureCount}}bg-green-100 text-green-800{{else}}bg-red-100 text-red-800{{end}}">
                            {{.SuccessCount}}/{{.TotalRequests}} Success
                        </span>
                        <span class="px-3 py-1 rounded-full bg-blue-100 text-blue-800">
                            {{printf "%.2f" .RequestsPerSecond}} RPS
                        </span>
                        {{if .IsConcurrent}}
                        <span class="px-3 py-1 rounded-full bg-indigo-100 text-indigo-800">
                            {{.ConcurrentUsers}} concurrent users
                        </span>
                        {{end}}
                    </div>
                </div>

                <!-- Performance Metrics -->
                <div class="grid grid-cols-3 gap-4 mb-4">
                    <div class="bg-white p-4 rounded shadow">
                        <h4 class="font-semibold mb-2">Latency</h4>
                        <div class="space-y-1">
                            <p>Min: {{.MinLatency}}</p>
                            <p>Max: {{.MaxLatency}}</p>
                            <p>Avg: {{.AverageLatency}}</p>
                            <p>P95: {{.Percentiles.P95}}</p>
                            <p>P99: {{.Percentiles.P99}}</p>
                        </div>
                    </div>
                    <div class="bg-white p-4 rounded shadow">
                        <h4 class="font-semibold mb-2">Response Sizes</h4>
                        <div class="space-y-1">
                            <p>Min: {{.ResponseSizes.Min}} bytes</p>
                            <p>Max: {{.ResponseSizes.Max}} bytes</p>
                            <p>Avg: {{.ResponseSizes.Avg}} bytes</p>
                            <p>Total: {{.BytesTransferred}} bytes</p>
                        </div>
                    </div>
                    <div class="bg-white p-4 rounded shadow">
                        <h4 class="font-semibold mb-2">Error Analysis</h4>
                        <div class="space-y-1">
                            <p>Error Rate: {{printf "%.2f" .ErrorRate}}%</p>
                            <p>Timeouts: {{.TimeoutCount}}</p>
                            <p>Validation Failures: {{len .ValidationFailures}}</p>
                        </div>
                    </div>
                </div>

                <!-- Status Code Distribution -->
                <div class="mb-4">
                    <h4 class="font-semibold mb-2">Status Codes</h4>
                    <div class="grid grid-cols-5 gap-2">
                        {{range $code, $count := .StatusCodes}}
                        <div class="bg-white p-2 rounded shadow text-center">
                            <span class="font-mono">{{$code}}</span>
                            <span class="block text-sm text-gray-600">{{$count}} requests</span>
                        </div>
                        {{end}}
                    </div>
                </div>

                <!-- Error Details -->
                {{if .Errors}}
                <div class="mb-4">
                    <h4 class="font-semibold text-red-600 mb-2">Errors</h4>
                    <div class="bg-white p-4 rounded shadow">
                        <ul class="list-disc list-inside space-y-1">
                            {{range .Errors}}
                            <li class="text-red-600">{{.}}</li>
                            {{end}}
                        </ul>
                    </div>
                </div>
                {{end}}

                <!-- Request Timeline -->
{{if .RequestDetails}}
<div>
    <h4 class="font-semibold mb-2">Request Timeline</h4>
    <div class="bg-white p-4 rounded shadow overflow-x-auto">
        <table class="min-w-full" id="requestTable-{{.EndpointName}}">
            <thead>
                <tr>
                    <th class="px-4 py-2 cursor-pointer" onclick="sortTable('requestTable-{{.EndpointName}}', 0)">ID ↕</th>
                    <th class="px-4 py-2 cursor-pointer" onclick="sortTable('requestTable-{{.EndpointName}}', 1)">Time ↕</th>
                    <th class="px-4 py-2 cursor-pointer" onclick="sortTable('requestTable-{{.EndpointName}}', 2)">Duration ↕</th>
                    <th class="px-4 py-2 cursor-pointer" onclick="sortTable('requestTable-{{.EndpointName}}', 3)">Status ↕</th>
                    <th class="px-4 py-2 cursor-pointer" onclick="sortTable('requestTable-{{.EndpointName}}', 4)">Size ↕</th>
                </tr>
            </thead>
            <tbody>
                {{range .RequestDetails}}
                <tr class="{{if .Success}}bg-green-50{{else}}bg-red-50{{end}}">
                    <td class="px-4 py-2" data-value="{{.ID}}">{{.ID}}</td>
                    <td class="px-4 py-2" data-value="{{.Timestamp.Unix}}">{{.Timestamp.Format "15:04:05.000"}}</td>
                    <td class="px-4 py-2" data-value="{{.Duration.Nanoseconds}}">{{.Duration}}</td>
                    <td class="px-4 py-2" data-value="{{.StatusCode}}">{{.StatusCode}}</td>
                    <td class="px-4 py-2" data-value="{{.ResponseSize}}">{{.ResponseSize}} bytes</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
</div>
{{end}}
            </div>
            {{end}}
        </div>
    </div>

    <script>
    function sortTable(tableId, columnIndex) {
        const table = document.getElementById(tableId);
        const tbody = table.getElementsByTagName('tbody')[0];
        const rows = Array.from(tbody.getElementsByTagName('tr'));
        let isAscending = table.getAttribute('data-sort-' + columnIndex) !== 'asc';
        
        rows.sort((a, b) => {
            let aValue = a.getElementsByTagName('td')[columnIndex].getAttribute('data-value');
            let bValue = b.getElementsByTagName('td')[columnIndex].getAttribute('data-value');
            
            // Convert to numbers if possible
            if (!isNaN(aValue) && !isNaN(bValue)) {
                aValue = Number(aValue);
                bValue = Number(bValue);
            }
            
            if (aValue < bValue) return isAscending ? -1 : 1;
            if (aValue > bValue) return isAscending ? 1 : -1;
            return 0;
        });
        
        // Update sort direction
        table.setAttribute('data-sort-' + columnIndex, isAscending ? 'asc' : 'desc');
        
        // Update table content
        rows.forEach(row => tbody.appendChild(row));
        
        // Update sorting indicators in header
        const headers = table.getElementsByTagName('th');
        Array.from(headers).forEach((header, index) => {
            header.textContent = header.textContent.replace(' ↑', '').replace(' ↓', '');
            if (index === columnIndex) {
                header.textContent += isAscending ? ' ↑' : ' ↓';
            } else {
                header.textContent += ' ↕';
            }
        });
    }



    // Calculate average latency
    const latencyValues = {{.ChartData.LatencyValues}};
    const avgLatency = latencyValues.reduce((a, b) => a + b, 0) / latencyValues.length;

    // Latency Chart with average line
    new Chart(document.getElementById('latencyChart').getContext('2d'), {
        type: 'scatter',
        data: {
            labels: {{.ChartData.Labels}},
            datasets: [{
                label: 'Latency (ms)',
                data: latencyValues.map((value, index) => ({
                    x: index,
                    y: value
                })),
                backgroundColor: 'rgb(75, 192, 192)',
                pointRadius: 6,
                pointHoverRadius: 8,
            }, {
                label: 'Average Latency',
                data: latencyValues.map((_, index) => ({
                    x: index,
                    y: avgLatency
                })),
                type: 'line',
                borderColor: 'rgba(255, 99, 132, 1)',
                borderDash: [5, 5],
                pointRadius: 0,
                fill: false
            }]
        },
        options: {
            responsive: true,
            plugins: {
                title: {
                    display: true,
                    text: 'Latency Distribution with Average'
                }
            },
            scales: {
                x: {
                    type: 'linear',
                    position: 'bottom',
                    ticks: {
                        callback: function(value) {
                            return {{.ChartData.Labels}}[value];
                        }
                    },
                    title: {
                        display: true,
                        text: 'Endpoints'
                    }
                },
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'Latency (ms)'
                    }
                }
            }
        }
    });

    // Success Rate Chart
    new Chart(document.getElementById('successRateChart').getContext('2d'), {
        type: 'bar',
        data: {
            labels: {{.ChartData.Labels}},
            datasets: [{
                label: 'Success Rate (%)',
                data: {{.ChartData.SuccessRates}},
                backgroundColor: 'rgba(75, 192, 192, 0.2)',
                borderColor: 'rgb(75, 192, 192)',
                borderWidth: 1
            }, {
                label: 'Error Rate (%)',
                data: {{.ChartData.ErrorRates}},
                backgroundColor: 'rgba(255, 99, 132, 0.2)',
                borderColor: 'rgb(255, 99, 132)',
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            plugins: {
                title: {
                    display: true,
                    text: 'Success and Error Rates'
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    max: 100,
                    title: {
                        display: true,
                        text: 'Rate (%)'
                    }
                },
                x: {
                    title: {
                        display: true,
                        text: 'Endpoints'
                    }
                }
            }
        }
    });
    </script>
</body>
</html>
`
