FROM golang:1.13

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go get -u github.com/onsi/ginkgo/ginkgo && go get -u github.com/onsi/gomega/...

FROM docker:stable-dind
# Docker needs somewhere to put creds from docker login.
RUN wget https://github.com/docker/docker-credential-helpers/releases/download/v0.6.0/docker-credential-pass-v0.6.0-amd64.tar.gz && tar -xf docker-credential-pass-v0.6.0-amd64.tar.gz && chmod +x docker-credential-pass &&  mv docker-credential-pass /bin
ENV DOCKER_HOST=tcp://127.0.0.1:2375
ENV GOPROXY direct

# Install Go, Git and other dependencies so we can run ginkgo
RUN apk add --no-cache --virtual .build-deps bash gcc musl-dev openssl go git

# Copy the binary
ADD bin/local/ecs-preview-amd64 /bin/ecs-preview

# Add the e2e directory and the project go.mod
ADD e2e/ github.com/aws/amazon-ecs-cli-v2/e2e/
ADD go.mod github.com/aws/amazon-ecs-cli-v2/
ADD go.sum github.com/aws/amazon-ecs-cli-v2/

# Startup script which inits dockerd and then runs the e2e tests
COPY e2e/e2e.sh /bin/

ENTRYPOINT ["/bin/e2e.sh"]
