FROM public.ecr.aws/docker/library/golang:1.20 as builder

ENV GOPROXY=direct
WORKDIR /go/src/app

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 go build -o /go/bin/app ./

FROM gcr.io/distroless/static

COPY --from=builder /go/bin/app /
EXPOSE 80
ENTRYPOINT ["/app"]