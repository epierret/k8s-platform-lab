# syntax=docker/dockerfile
ARG GO_VERSION=1.27.4
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
ARG TARGET=app TARGETOS TARGETARCH VERSION=dev COMMIT=none DATE=unknown
WORKDIR /src
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY api/ api/
COPY cmd/ cmd/
COPY internal/ internal/
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /out/bin ./cmd/${TARGET}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bin /bin/entrypoint
USER 65532:65532
ENTRYPOINT ["/bin/entrypoint"]
