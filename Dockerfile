FROM golang:1.19.3-alpine AS BuildStage

ARG WATCH_NAMESPACES
WORKDIR /ng

COPY . .

RUN go mod download
RUN go build -o /app main.go

# Deploy Stage

FROM alpine:latest
WORKDIR /
COPY --from=BuildStage /app /app

# USER nonroot:nonroot

ENV WATCH_NAMESPACES=${WATCH_NAMESPACES:-ng}

ENTRYPOINT ["/app"]