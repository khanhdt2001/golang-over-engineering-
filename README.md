# User and task modules

A small Go HTTP API for user sign-up, sign-in, and profile updates. It stores users and hashed session tokens in PostgreSQL, and emits JSON request logs for Loki/Grafana.

## Start here

Requirements: Docker with Docker Compose, or Go 1.25+ and PostgreSQL.

Start the complete local stack:

```sh
docker compose up --build
```

Services exposed on the host:

| Service | Address | Purpose |
| --- | --- | --- |
| User API | `http://localhost:8080` | User HTTP API |
| Task API | `http://localhost:8081` | Task HTTP API |
| Task gRPC API | `localhost:9091` | Task gRPC API |
| Kafka logger | `docker compose logs -f kafka-logger` | Replays and logs Kafka audit messages |
| User PostgreSQL | `localhost:15432` | `users` database (`app` / `app`) |
| Task PostgreSQL | `localhost:25432` | `tasks` database (`app` / `app`) |
| Grafana | `http://localhost:3000` | Log exploration |
| Loki | `http://localhost:3100` | Log store |
| Alloy | `http://localhost:12345` | Docker-log collector status |

Run either API outside Docker (its Compose PostgreSQL must be running):

```sh
docker compose up -d user-postgres task-postgres
cd user && go run .
# separately, with task-postgres running
cd task && go run .
```

`DATABASE_URL` overrides the default connection string:

```text
user: postgres://app:app@localhost:15432/users?sslmode=disable
task: postgres://app:app@localhost:25432/tasks?sslmode=disable
```

The API creates its tables and indexes on startup. Run checks with:

```sh
cd user && go test ./...
cd task && go test ./...
cd kafka-logger && go test ./...
```

The Kafka logger reads every message from `user-api-calls` and `task-api-calls`
from the first offset, then logs it as JSON. Override `KAFKA_BROKERS` or
`KAFKA_TOPICS` (a comma-separated list) when running it outside Compose.

Generate the Go gRPC bindings with Docker. The repository pins Buf and both Go
plugins, so developers do not need `protoc` or Go generator plugins installed:

```sh
./scripts/gen-proto.sh
```

The shared generated types live in the `proto` module. Import them from either
API as needed:

```go
import userpb "github.com/khanhdt2001/golang-over-engineering-/proto/user/v1"
import taskpb "github.com/khanhdt2001/golang-over-engineering-/proto/task/v1"
```

The user API also serves the `overengineering.user.v1.UserService` gRPC service
on `localhost:9090`. Its `SignUp`, `SignIn`, `CheckUser`, and
`UpdateProfile` messages are defined in `proto/user/v1/user.proto`; send the
session token as `authorization: Bearer <token>` metadata for `UpdateProfile`.

The task API serves `overengineering.task.v1.TaskService` on `localhost:9091`.
Its `CreateTask`, `GetTask`, `ListUserTasks`, and `UpdateTask` messages are
defined in `proto/task/v1/task.proto`.

Before creating or reassigning a task, the task API calls
`overengineering.user.v1.UserService.CheckUser` over gRPC. It uses
`USER_GRPC_ADDR` (default `localhost:9090`; Compose uses `user-api:9090`).

## API contract

All request and response bodies are JSON. Unknown request fields and malformed JSON return `400`.

### Create user

`POST /v1/users/sign-up`

```sh
curl -i http://localhost:8080/v1/users/sign-up \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"password","username":"Ada"}'
```

Success (`201`):

```json
{"id":"uuid","email":"ada@example.com","username":"Ada"}
```

### Sign in

`POST /v1/users/sign-in`

```sh
curl -s http://localhost:8080/v1/users/sign-in \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"password"}'
```

Success (`200`):

```json
{"token":"session-token","user":{"id":"uuid","email":"ada@example.com","username":"Ada"}}
```

The token is returned only at sign-in. Save it for authenticated requests.

### Update the current user

`PATCH /v1/users/me`

Send one or more fields to change. The bearer token is required.

```sh
curl -i http://localhost:8080/v1/users/me \
  -X PATCH \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"username":"Ada Lovelace"}'
```

Success (`200`):

```json
{"id":"uuid","email":"ada@example.com","username":"Ada Lovelace"}
```

### Errors and validation

Errors use `{"error":"..."}`.

| Status | When |
| --- | --- |
| `400` | Invalid JSON, unknown fields, empty update, invalid email, password, or username |
| `401` | Missing, invalid, or expired bearer token; invalid sign-in credentials |
| `409` | Email is already registered |
| `500` | Unexpected server or database error |

Validation rules:

- Email must be a parsed email address, unchanged by parsing, and at most 254 characters. It is stored and matched case-insensitively.
- Password must be 8–72 characters. Passwords are bcrypt-hashed and never returned.
- Username must contain at least one non-whitespace character and be at most 100 characters.
- Sessions last 24 hours. Only the SHA-256 hash of a session token is stored.

## Debugging guide

The request middleware writes one JSON line per request to standard output. A normal entry looks like:

```json
{"level":"INFO","msg":"request complete","method":"POST","path":"/v1/users/sign-in","status":200,"duration":"1.2ms"}
```

Client and server errors log at `ERROR`; handled service errors also include `error`. For a quick local view:

```sh
docker compose logs -f user-api
```

For persisted logs, open Grafana at `http://localhost:3000`, select the **Loki** data source, and query:

```logql
{service="user-api"}
```

Useful filters:

```logql
{service="user-api"} | json | status >= 400
{service="user-api"} | json | path = "/v1/users/me"
{service="user-api"} | json | error != ""
{service="kafka-logger"}
```

## Task API contract

Task fields are `id`, `name`, `description`, `time`, and `user_id`. The task module stores them in its own `tasks` database. PostgreSQL foreign keys cannot span the separate `users` and `tasks` databases, so the task module validates `user_id` with the user module's `CheckUser` gRPC RPC before creating or reassigning a task.

### Create task

`POST /v1/tasks`

```sh
curl -i http://localhost:8081/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{"name":"Write docs","description":"Document the task API","time":"2026-08-31T09:00:00Z","user_id":"USER_UUID"}'
```

Success: `201` with the created task.

### Get task

`GET /v1/tasks/{id}` returns the task, or `404` when it does not exist.

### List a user's tasks

`GET /v1/users/{user_id}/tasks` returns an array ordered by `time`.

### Update task

`PATCH /v1/tasks/{id}` accepts one or more of `name`, `description`, `time`, and `user_id`.

```sh
curl -i http://localhost:8081/v1/tasks/TASK_UUID \
  -X PATCH \
  -H 'Content-Type: application/json' \
  -d '{"name":"Write task documentation"}'
```

Task request bodies reject unknown fields. Names must be non-blank and at most 100 characters; descriptions are at most 1000 characters; `time` must be RFC 3339; IDs must be UUIDs. Invalid input returns `400`; a missing task returns `404`; unexpected errors return `500`.

Debug the module with:

```sh
docker compose logs -f task-api
```

If the API does not start, check database reachability and migrations:

```sh
docker compose ps
docker compose logs user-postgres user-api
```

If Grafana has no user logs, ensure Alloy is healthy and that the API container name is still the Compose service `user-api`; the Alloy configuration deliberately selects only that service.

## Code map

```text
HTTP request
  -> user/api       JSON decoding, routing, HTTP errors, request logs
  -> user/service   validation, password hashing, sessions, business errors
  -> user/db        PostgreSQL queries and schema migration
```

- `user/main.go` wires the database, migration, service, and HTTP handler.
- `user/api/http.go` is the public HTTP boundary. Add routes and response mapping here.
- `user/service/user.go` owns business rules. It depends on the small `Repository` interface, so unit tests can use a fake.
- `user/db/postgres.go` is the only PostgreSQL implementation and owns the tables, indexes, and SQL.
- `compose.yaml` runs both APIs, their own PostgreSQL databases, Kafka, and the observability stack.

## Pattern for the next module

Use this layout and keep the ownership boundaries intact:

```text
<module>/
  main.go           # configuration and dependency wiring only
  api/http.go        # transport: routes, JSON, auth header extraction, HTTP mapping
  service/<name>.go  # use cases, validation, domain errors, repository interface
  db/postgres.go     # migrations and SQL implementation
  api/*_test.go      # HTTP/logging behavior
  service/*_test.go  # business behavior with a fake repository
  Dockerfile
```

Before calling a new module done:

- Document its routes, sample requests, error statuses, validation, configuration, and one end-to-end debug command.
- Keep routes thin: parse and map HTTP in `api`; validate and decide in `service`; access PostgreSQL in `db`.
- Define only the repository methods the service currently calls; do not add a generic data-access layer.
- Emit structured JSON logs to stdout and give the container a stable Compose service name. Add a matching Alloy selector only when that module needs Loki logs.
- Add the smallest tests that protect its business boundary and public HTTP behavior, then run `go test ./...` from the module directory.
