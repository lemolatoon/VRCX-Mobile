FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /collector ./cmd/collector

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /collector /collector
ENTRYPOINT ["/collector"]
