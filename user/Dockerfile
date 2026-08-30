FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /user-api .

FROM alpine:3.22
COPY --from=build /user-api /user-api
EXPOSE 8080
ENTRYPOINT ["/user-api"]
