.PHONY: examples/example-rules.yaml

example-rules.yaml:
	go run cmd/slo-builder/main.go build examples/*-slo.yaml > examples/rules.yaml
