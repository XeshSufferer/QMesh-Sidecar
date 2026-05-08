.PHONY: build docker-build docker-push deploy clean

BINARY_NAME=sidecar
IMAGE_NAME=qmesh-sidecar
IMAGE_TAG?=latest
REGISTRY?=localhost:5000

build:
	CGO_ENABLED=0 GOOS=linux go build -o build/$(BINARY_NAME) ./cmd/main.go

docker-build:
	docker build -t $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG) .

docker-push:
	docker push $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

deploy:
	kubectl apply -f deploy/k8s/qmesh-headless-service.yaml
	kubectl apply -f deploy/k8s/sidecar-rbac.yaml
	kubectl apply -f deploy/k8s/example-deployment.yaml

deploy-statefulset:
	kubectl apply -f deploy/k8s/qmesh-headless-service.yaml
	kubectl apply -f deploy/k8s/sidecar-rbac.yaml
	kubectl apply -f deploy/k8s/example-statefulset.yaml

clean:
	rm -f build/$(BINARY_NAME)

test:
	go test ./...

lint:
	golangci-lint run ./...
