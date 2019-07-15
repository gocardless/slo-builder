.PHONY: slo-alerts.yaml

slo-alerts.yaml:
	go run . > $@
