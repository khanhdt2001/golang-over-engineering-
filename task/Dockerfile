FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /task-api .

FROM alpine:3.22
COPY --from=build /task-api /task-api
EXPOSE 8081
ENTRYPOINT ["/task-api"]
