# Production image (ADR 0008): built with the official Go image, run from
# distroless static as nonroot: a static binary, no shell, no package manager.
# Both bases are pinned by digest; Dependabot's docker ecosystem moves them.
# The release rehearsal in CI builds, runs, smoke-tests and scans this image.
FROM golang:1.27.3-bookworm@sha256:dc0963f32053e4bc451d24d87e3892d42d01021b61f093e98eb95c0352bb84b0 AS build
WORKDIR /src
# A toolchain bump in go.mod above the pinned base fails the build rather than
# downloading a toolchain the digest does not cover.
ENV GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /workspots ./cmd/workspots

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /workspots /workspots
# ADDR defaults to 127.0.0.1:8080, which nothing outside the container can
# reach; set ADDR=:8080 (or bind to the proxy's network) when you run it.
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/workspots"]
