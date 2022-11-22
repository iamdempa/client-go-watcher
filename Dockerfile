FROM golang:1.19.3-alpine AS BuildStage

ARG NAMESPACE_TO_WATCH
ARG other_namespace_to_watch
ARG KUBECONFIG
WORKDIR /ng

COPY . .

RUN go mod download
RUN go build -o /app main.go

# Deploy Stage
FROM alpine:latest
WORKDIR /
COPY --from=BuildStage /app /app

ENV NAMESPACE_TO_WATCH=${NAMESPACE_TO_WATCH:-default}
ENV other_namespace_to_watch=${other_namespace_to_watch:-default}
ENV KUBECONFIG=${KUBECONFIG}

ENTRYPOINT ["/app"]