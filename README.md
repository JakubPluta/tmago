# TMAGO

## Project Overview

**TMAGO** is a powerful API testing tool written in Go, designed to validate and monitor API performance. It allows users to configure, execute, and analyze API tests with customizable settings such as retry logic, concurrent users, and response validation. The results are collected and presented through detailed HTML reports, which include performance metrics like request success rates, latency, and response sizes.

With TMAGO, you can automate the testing of RESTful APIs, ensuring that they meet specified performance and functionality requirements.

## Features

- **API Endpoint Testing**: Test multiple API endpoints with different HTTP methods and configurations.
- **Retry Mechanism**: Retry failed requests automatically based on your configuration.
- **Concurrent Testing**: Simulate multiple users making requests simultaneously.
- **Response Validation**: Validate the HTTP status code, response time, and body content of API responses.
- **HTML Report Generation**: Generate a comprehensive HTML report with key performance metrics and visualizations.
- **Logging**: Extensive logging of test progress, errors, and results.

## Project Structure

## Installation

To install **TMAGO**, follow these steps:

### 1. Clone the repository:
```bash
git clone https://github.com/JakubPluta/tmago.git
```

### 2. Install dependencies:
```
cd tmago
go mod tidy
```

### 3. Build the executable
```bash
go build -o tmago

```

### 4. Run the tool:
```bash
./tmago --config path/to/config.yaml

```

## Configuration
The configuration is defined in a YAML file. Below is an example of the configuration file:

```yaml
endpoints:
  - name: "Json PlaceHolder"
    url: "https://jsonplaceholder.typicode.com/posts/1"
    method: "GET"
    expect:
      status: 200
      maxTime: "500ms"
      values:
        - path: "title"
          value: "foo"
    retry:
      count: 1
      delay: "1s"
    concurrent:
      users: 10
      delay: "20ms"
      total: 50
```


### In the configuration file:

- **endpoints**: A list of API endpoints to test.
- **name**: A friendly name for the endpoint.
- **url**: The URL of the API endpoint.
- **method**: The HTTP method to use (e.g., GET, POST).
- **headers**: Optional HTTP headers to include in the request.
- **body**: The request body for methods like POST.
- **expect**: The expected response status and values (e.g., JSON path checks).
- **retry**: Configures the retry logic (number of attempts and delay).
- **concurrent**: Specifies the number of concurrent users, request delay, and total requests to simulate.

concurrency configuration
```yaml
concurrent:
    users: 5
    delay: 2s
    total: 50
```
5 concurrent users will be simulated. Each user will have a 2-second delay between requests. The test will send a total of 50 requests to the configured endpoint, meaning the requests will be distributed among the 5 users, and each will send 10 requests (total/5 = 10 requests per user).
