FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-X main.buildVersion=${VERSION}" -o /out/agent-container-hub ./cmd/agent-container-hub

FROM alpine:3.22

WORKDIR /app
ENV SERVER_HOST=0.0.0.0
ENV SERVER_PORT=8080
ENV ENGINE=auto

COPY --from=builder /out/agent-container-hub /usr/local/bin/agent-container-hub
COPY .env.example /app/.env.example
COPY configs/hub.example.yml /app/configs/hub.example.yml

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/agent-container-hub"]
