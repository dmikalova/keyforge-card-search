# Build the binary. The templ output is committed, and the static files are
# embedded, so the Go build is the whole build. CGO is off so the result runs
# on a distroless static image.
FROM golang:1.27 AS build

WORKDIR /src

# Dependencies are copied first so a source-only change reuses the module layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/search ./cmd/search

# Run as a non-root user with no shell and no package manager.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/search /search

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/search"]
