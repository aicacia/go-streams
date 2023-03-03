.PHONY: docker-build
.DEFAULT_GOAL := docker-build

run:
	go install github.com/mitranim/gow@latest && gow -v run *.go -- ./config.properties

docker-build:
	docker build -f Dockerfile -t aicacia-streams:0.0.0-alpine3.17 .

docker-run:
	docker run --rm --network=host aicacia-streams:0.0.0-alpine3.17 ./config.properties