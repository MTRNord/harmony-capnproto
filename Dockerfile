#syntax=docker/dockerfile:1.2

#
# base installs required dependencies and runs go mod download to cache dependencies
#
FROM --platform=${BUILDPLATFORM} docker.io/golang:alpine AS base
RUN apk --update --no-cache add bash build-base curl git capnproto

#
# Install the capnproto go requirements and codegen the files
# We use main of it as the latest is broken right now due to go packaging system
#
RUN go install capnproto.org/go/capnp/v3/capnpc-go@main
RUN git clone https://github.com/capnproto/go-capnp /go-capnp
RUN capnp compile -I/go-capnp/std --verbose -ogo ./**/*.capnp

#
# build creates all needed binaries
#
FROM --platform=${BUILDPLATFORM} base AS build
WORKDIR /src
ARG TARGETOS
ARG TARGETARCH
RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    USERARCH=`go env GOARCH` \
    GOARCH="$TARGETARCH" \
    GOOS="linux" \
    CGO_ENABLED=$([ "$TARGETARCH" = "$USERARCH" ] && echo "1" || echo "0") \
    go build -v -ldflags="-s -w" -trimpath -o /out/ ./cmd/...


#
# Builds the Dendrite image containing all required binaries
#
FROM alpine:latest
RUN apk --update --no-cache add curl
LABEL org.opencontainers.image.title="Harmony"
LABEL org.opencontainers.image.description="Fork of the Dendrite Matrix homeserver"
LABEL org.opencontainers.image.source="https://github.com/neilalexander/harmony"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.documentation="https://matrix-org.github.io/dendrite/"
LABEL org.opencontainers.image.vendor="Neil Alexander"

COPY --from=build /out/create-account /usr/bin/create-account
COPY --from=build /out/generate-config /usr/bin/generate-config
COPY --from=build /out/generate-keys /usr/bin/generate-keys
COPY --from=build /out/dendrite /usr/bin/dendrite

VOLUME /etc/dendrite
WORKDIR /etc/dendrite

ENTRYPOINT ["/usr/bin/dendrite"]
EXPOSE 8008 8448

