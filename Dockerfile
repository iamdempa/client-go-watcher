# FROM golang:1.19.3-alpine AS build

# WORKDIR /app

# COPY go.mod ./
# COPY go.sum ./
# RUN go mod download

# COPY *.go ./

# ARG WATCH_NAMESPACES

# RUN go build -o /docker-gs-ping

# ## Deploy
# FROM gcr.io/distroless/base-debian10

# WORKDIR /

# COPY --from=build /docker-gs-ping /docker-gs-ping

# ENV WATCH_NAMESPACES=${WATCH_NAMESPACES:-ng}

# USER nonroot:nonroot

# ENTRYPOINT ["/docker-gs-ping"]


FROM golang:1.19.3-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /app1

# EXPOSE 8080

CMD [ "/app1" ]