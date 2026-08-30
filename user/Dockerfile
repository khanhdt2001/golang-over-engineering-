FROM golang:1.25-alpine AS build
WORKDIR /src
COPY proto/go.mod proto/go.sum ./proto/
COPY user/go.mod user/go.sum ./user/
WORKDIR /src/user
RUN go mod download
WORKDIR /src
COPY proto ./proto
COPY user ./user
WORKDIR /src/user
RUN CGO_ENABLED=0 go build -o /user-api .

FROM alpine:3.22
COPY --from=build /user-api /user-api
EXPOSE 8080
EXPOSE 9090
ENTRYPOINT ["/user-api"]
