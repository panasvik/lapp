# Keep the CGO build and runtime on the same Alpine release.
ARG ALPINE_VERSION=3.23

FROM golang:1.26-alpine${ALPINE_VERSION} AS builder

WORKDIR /app

RUN apk add --no-cache \
    ca-certificates \
    gcc \
    musl-dev \
    pkgconf \
    vips-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# bimg links against libvips; go-sqlite3 compiles its bundled SQLite via CGO.
RUN CGO_ENABLED=1 GOOS=linux go build -o /my-app ./cmd/app

FROM alpine:${ALPINE_VERSION}

WORKDIR /

# HEIC support is a separate libvips module. It pulls in libheif and the
# libde265 decoder / x265 encoder libraries as dependencies.
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    vips \
    vips-heif

# The application uses paths relative to its working directory.
RUN mkdir -p \
    /assets/cache/lib \
    /assets/cache/manifest \
    /assets/cache/modal \
    /assets/uploads

COPY --from=builder /my-app /my-app

#COPY --from=builder /app/loveApp.db /loveApp.db

EXPOSE 8080

ENTRYPOINT ["/my-app"]
CMD []