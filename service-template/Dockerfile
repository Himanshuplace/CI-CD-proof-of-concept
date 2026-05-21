# Copy this Dockerfile into each Go service and adjust CMD_PATH if needed.
ARG GO_VERSION=1.22
ARG CMD_PATH=./cmd/server

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/service \
    ${CMD_PATH}

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/service /app/service
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/service"]

