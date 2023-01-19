STAGE ?= dev

include env.$(STAGE)
include .env
export

.SILENT:
SHELL = /bin/bash

# Set USER_ID and GROUP_ID to current user's uid and gid if not set
USER_ID ?= $(shell id -u)
GROUP_ID ?= $(shell id -g)

build: fmt ## Build Docker image
	echo "Building frontend ..."
	cd ../rdm-integration-frontend && rm -rf ./dist && ng build --configuration="production" --base-href /integration/
	echo "Building Docker image ..."
	rm -rf image/app/frontend/dist
	cp -r ../rdm-integration-frontend/dist image/app/frontend/dist
	docker build \
		--build-arg USER_ID=$(USER_ID) --build-arg GROUP_ID=$(GROUP_ID) \
		--tag "$(IMAGE_TAG)" ./image

push: ## Push Docker image (only in prod stage)
	if [ "$(STAGE)" = "prod" ]; then \
		echo "Pushing Docker image to repository ..."; \
		docker push $(IMAGE_TAG); \
	else \
		echo "Not in production stage. Pushing not allowed."; \
	fi

run: fmt ## Run the server locally
	echo "Building frontend ..."
	cd ../rdm-integration-frontend && rm -rf ./dist && ng build --configuration development
	rm -rf image/app/frontend/dist
	cp -r ../rdm-integration-frontend/dist image/app/frontend/dist
	echo "Starting redis ..."
	docker stop redis || true && docker rm redis || true && docker run -p 6379:6379 --name redis -d redis
	echo "Starting app ..."
	cd image && go run ./app 100

fmt: ## Format the go code
	cd image && go fmt ./app/...

frontend: ## build frontend
	echo "Building frontend ..."
	cd ../rdm-integration-frontend && rm -rf ./dist && ng build --configuration="production"
	rm -rf image/app/frontend/dist
	cp -r ../rdm-integration-frontend/dist image/app/frontend/dist

executable: fmt frontend ## build executable for running locally, e.g. cd image && go build -ldflags "-X main.DataverseServer=https://demo.dataverse.org -X main.RootDataverseId=demo -X main.DefaultHash=MD5" -v -o datasync.exe ./app/local/
	cp -r conf/customizations/* image/app/frontend/dist/datasync/
	cd image && go build -ldflags '-s -w -X main.DataverseServer=https://demo.dataverse.org -X "main.DataverseServerName=Demo Dataverse"' -v -o ../datasync.exe ./app/local/

multiplatform_demo: fmt frontend ## build executable for multiple platforms
	cp -r conf/customizations/* image/app/frontend/dist/datasync/
	cd image && env GOOS=windows GOARCH=amd64 go build -ldflags '-s -w -X main.DataverseServer=https://demo.dataverse.org -X "main.DataverseServerName=Demo Dataverse"' -v -o demo_win.exe ./app/local/
	cd image && env GOOS=linux GOARCH=amd64 go build -ldflags '-s -w -X main.DataverseServer=https://demo.dataverse.org -X "main.DataverseServerName=Demo Dataverse"' -v -o demo_linux.bin ./app/local/
	cd image && env GOOS=darwin GOARCH=amd64 go build -ldflags '-s -w -X main.DataverseServer=https://demo.dataverse.org -X "main.DataverseServerName=Demo Dataverse"' -v -o demo_darwin_amd64.bin ./app/local/
	cd image && env GOOS=darwin GOARCH=arm64 go build -ldflags '-s -w -X main.DataverseServer=https://demo.dataverse.org -X "main.DataverseServerName=Demo Dataverse"' -v -o demo_darwin_arm64.bin ./app/local/