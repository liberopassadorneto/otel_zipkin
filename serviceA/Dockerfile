FROM golang:1.23.5-alpine

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download && go mod tidy

RUN go get go.opentelemetry.io/otel \
    && go get go.opentelemetry.io/otel/attribute \
    && go get go.opentelemetry.io/otel/exporters/zipkin \
    && go get go.opentelemetry.io/otel/sdk/trace

COPY *.go ./

RUN go build -o serviceA

CMD ["./serviceA"]
