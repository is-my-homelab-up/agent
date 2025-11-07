main_package_path = .
binary_name = agent

.PHONY: tidy
tidy:
	go mod tidy -v
	go fmt ./...

.PHONY: test
test:
	go test -v -race -buildvcs ./...

.PHONY: build
build:
	go build -o=bin/${binary_name} ${main_package_path}

.PHONY: run
run: build
	bin/${binary_name}

