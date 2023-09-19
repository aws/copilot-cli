FROM golang:1.21
# We need to have both nodejs and go to build the binaries.
# We could use multi-stage builds but that would require significantly changing our Makefile.
RUN apt-get update
RUN curl -sL https://deb.nodesource.com/setup_10.x | bash -
RUN apt-get update && apt-get install -y nodejs

WORKDIR /copilot
COPY . .
RUN go env -w GOPROXY=direct
RUN make release
