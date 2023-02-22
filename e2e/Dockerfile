FROM docker:20.10.13-dind

ARG GOLANG_VERSION=1.20

# Docker needs somewhere to put creds from docker login.
RUN wget https://github.com/docker/docker-credential-helpers/releases/download/v0.6.0/docker-credential-pass-v0.6.0-amd64.tar.gz && tar -xf docker-credential-pass-v0.6.0-amd64.tar.gz && chmod +x docker-credential-pass &&  mv docker-credential-pass /bin
ENV DOCKER_HOST=tcp://127.0.0.1:2375
ENV GOBIN /go/bin

# Install Go, Git and other dependencies so we can run ginkgo
RUN apk update && apk add bash gcc musl-dev openssl go git aws-cli jq

# Upgrade go to $GOLANG_VERSION. The version that's available in the base image is "go1.13.15 linux/amd64" by default.
RUN wget https://dl.google.com/go/go$GOLANG_VERSION.src.tar.gz && tar -C /usr/local -xzf go$GOLANG_VERSION.src.tar.gz
RUN cd /usr/local/go/src && ./make.bash
ENV PATH=$PATH:/usr/local/go/bin
RUN rm go$GOLANG_VERSION.src.tar.gz
RUN apk del go

RUN go install github.com/onsi/ginkgo/v2/ginkgo@latest

# Copy the binary
ADD bin/local/copilot-linux-amd64 /bin/copilot

# Add the e2e directory and the project go.mod
ADD e2e/ github.com/aws/copilot-cli/e2e/
ADD go.mod github.com/aws/copilot-cli/
ADD go.sum github.com/aws/copilot-cli/

# Startup script which inits dockerd and then runs the e2e tests
COPY e2e/e2e.sh /bin/

ENTRYPOINT ["/bin/e2e.sh"]
