# Simple usage with a mounted data directory:
# > docker build -t simapp .
#
# Server:
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.simapp:/root/.simapp simapp simd init test-chain
# TODO: need to set validator in genesis so start runs
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.simapp:/root/.simapp simapp simd start
#
# Client: (Note the simapp binary always looks at ~/.simapp we can bind to different local storage)
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.simappcli:/root/.simapp simapp simd keys add foo
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.simappcli:/root/.simapp simapp simd keys list
#
# This image is pushed to the GHCR as https://ghcr.io/cosmos/simapp

FROM golang:1.27.2-alpine AS build-env

# Install minimum necessary dependencies
ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev
RUN apk add --no-cache $PACKAGES

# Set working directory for the build
WORKDIR /go/src/github.com/cosmos/cosmos-sdk

# optimization: if go.sum didn't change, docker will use cached image
COPY go.mod go.sum ./
COPY collections/go.mod collections/go.sum ./collections/
COPY log/go.mod log/go.sum ./log/
# Every module the root go.mod replaces with a local directory.
COPY api/go.mod api/go.sum ./api/
COPY store/go.mod store/go.sum ./store/
COPY x/evidence/go.mod x/evidence/go.sum ./x/evidence/
COPY x/feegrant/go.mod x/feegrant/go.sum ./x/feegrant/
COPY x/tx/go.mod x/tx/go.sum ./x/tx/
COPY x/upgrade/go.mod x/upgrade/go.sum ./x/upgrade/

# Private InjectiveLabs modules come from the file proxy staged on the host by
# scripts/prepare-docker-go-modules.sh, so no credentials enter the image.
COPY .modcache/ /modcache/
ENV GOPROXY=file:///modcache,https://proxy.golang.org,direct \
    GONOPROXY=none \
    GONOSUMDB=github.com/InjectiveLabs
RUN go mod download

# Add source files
COPY . .

# Dockerfile Cross-Compilation Guide
# https://www.docker.com/blog/faster-multi-platform-builds-dockerfile-cross-compilation-guide
ARG TARGETOS TARGETARCH

# install simapp, remove packages
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH make build

# Use alpine:3 as a base image
FROM alpine:3

EXPOSE 26656 26657 1317 9090
# Run simd by default, omit entrypoint to ease using container with simcli
CMD ["simd"]
STOPSIGNAL SIGTERM
WORKDIR /root

# Install minimum necessary dependencies
RUN apk add --no-cache curl make bash jq sed

# Copy over binaries from the build-env
COPY --from=build-env /go/src/github.com/cosmos/cosmos-sdk/build/simd /usr/bin/simd
