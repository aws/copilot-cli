# We specify the base image we need for our
# go application
FROM public.ecr.aws/docker/library/golang:1.20 as builder
# We create an /app directory within our
# image that will hold our application source
# files
RUN mkdir /app
# We copy everything in the root directory
# into our /app directory
ADD . /app
# We specify that we now wish to execute
# any further commands inside our /app
# directory
WORKDIR /app
# Avoid the GoProxy
ENV GOPROXY=direct
# we run go build to compile the binary
# executable of our Go program
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64  go build -o e2e-service ./

# To make our images smaller, use alpine and copy in the service binary.
FROM alpine:latest
# curl is needed for container healthchecks.
RUN apk update
RUN apk add curl
# Install certs
RUN apk --no-cache add ca-certificates
# Copy the binary from the builder image
COPY --from=builder /app ./
# Make the binary executable
RUN chmod +x ./e2e-service
# Start the service
ENTRYPOINT ["./e2e-service"]
# The service runs on port 80
EXPOSE 80
# The service performs a health check every 1 minute
HEALTHCHECK --interval=1m --timeout=3s \
  CMD curl -f http://localhost/ || exit 1
