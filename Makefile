.PHONY: compile run

compile:
	- go build -o bin/tmago

run-example:
	- ./bin/tmago run --config example.yaml

