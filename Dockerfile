FROM golang:1.25-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum* ./

# Drop local replace directive; fetch private core module via injected token
RUN --mount=type=secret,id=github_token \
    git config --global url."https://x-access-token:$(cat /run/secrets/github_token)@github.com/".insteadOf "https://github.com/" && \
    GOPRIVATE=github.com/wisbric/* go mod edit -dropreplace=github.com/wisbric/core && \
    go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/nightowl ./cmd/nightowl

# ---
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/nightowl /nightowl
COPY migrations/ /migrations/

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/nightowl"]
