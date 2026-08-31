FROM golang:1.25-alpine AS build
WORKDIR /src/kafka-logger
COPY kafka-logger/go.mod ./
RUN go mod download
COPY kafka-logger ./
RUN CGO_ENABLED=0 go build -o /kafka-logger .

FROM alpine:3.22
COPY --from=build /kafka-logger /kafka-logger
ENTRYPOINT ["/kafka-logger"]
