# gitta-git-service

Git smart HTTP service for Gitta.

## Development

Required environment:

```txt
GIT_SERVICE_INTERNAL_SECRET=dev-git-service-secret
API_INTERNAL_URL=http://localhost:3000
REPO_ROOT=.data/repos
PORT=4001
LOG_LEVEL=debug
```

For local development:

```sh
cp .env.example .env
```

Run locally:

```sh
go run ./cmd/server
```

Run tests:

```sh
go test ./...
```

In restricted sandboxes, integration tests may fail if local TCP listeners are blocked. The integration package starts local `httptest` servers and runs real `git` commands.
