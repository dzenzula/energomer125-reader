
BUILD_NAME ?= energomer125-reader
DEST_DIR := deploy/
DEPLOY_FILES := ${BUILD_NAME} configs/${BUILD_NAME}.conf.yml service/${BUILD_NAME}.service

build:
	go build -ldflags "-s -w" -o ${BUILD_NAME} cmd/energomer125-reader/main.go

run:
	./${BUILD_NAME}

#build_and_run: build run

compile:
	GOARCH=amd64 GOOS=darwin go build -ldflags "-s -w" -o ${BUILD_NAME} cmd/energomer125-reader/main.go
	GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o ${BUILD_NAME} cmd/energomer125-reader/main.go
	GOARCH=amd64 GOOS=window go build -ldflags "-s -w" -o ${BUILD_NAME} cmd/energomer125-reader/main.go

install: ${DEPLOY_FILES}
	mkdir -p ${DEST_DIR}
	for f in ${DEPLOY_FILES}; do echo $$f;  cp -f $$f ${DEST_DIR}; done

clean:
	go clean
	rm -r ${DEST_DIR}
