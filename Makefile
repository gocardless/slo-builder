.PHONY: example-rules.yaml

example-rules.yaml:
	go run cmd/slo-builder/main.go build example-definitions.yaml > $@
