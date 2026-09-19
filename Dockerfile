# syntax=docker/dockerfile:1

# --- build stage ---
FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Which service under ./cmd to build: payment-service, payment-worker,
# or fake-gateway.
ARG SERVICE
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/${SERVICE}

# --- run stage ---
FROM alpine:3.20

WORKDIR /app
COPY --from=build /app/server .

CMD ["./server"]
