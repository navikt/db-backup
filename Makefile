NAME := db-backup
REPO := navikt/${NAME}
TAG := $(shell date +'%Y-%m-%d')-$(shell git rev-parse --short HEAD)

.PHONY: docker-build docker-push

all: docker-build docker-push

docker-build:
	docker build -t "$(REPO):$(TAG)" -t "$(REPO):latest" .

docker-push:
	docker push "$(REPO)"