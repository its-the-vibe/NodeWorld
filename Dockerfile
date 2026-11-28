FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /nodeworld .

FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app
COPY --from=builder /nodeworld /nodeworld
COPY examples/sample.yaml examples/sample.yaml

USER nonroot:nonroot

ENTRYPOINT ["/nodeworld"]
