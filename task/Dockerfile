FROM golang:1.25-alpine AS build
WORKDIR /src
COPY proto/go.mod proto/go.sum ./proto/
COPY task/go.mod task/go.sum ./task/
WORKDIR /src/task
RUN go mod download
WORKDIR /src
COPY proto ./proto
COPY task ./task
WORKDIR /src/task
RUN CGO_ENABLED=0 go build -o /task-api .

FROM alpine:3.22
COPY --from=build /task-api /task-api
EXPOSE 8081
EXPOSE 9091
ENTRYPOINT ["/task-api"]
