FROM golang:1.15.8 AS builder
WORKDIR /go/src/github.com/coreos/etcd-operator

ARG VERSION=dev
ARG REVISION=dev
ARG CREATED=dev

COPY cmd cmd
COPY pkg pkg
COPY version version
COPY go.mod go.sum ./

# Produce a static / reproducible build
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
RUN go mod tidy && \
    go mod vendor
RUN go build --ldflags "-w -s -X 'github.com/coreos/etcd-operator/version.GitSHA=$REVISION'" -o /usr/local/bin/etcd-operator github.com/coreos/etcd-operator/cmd/operator
RUN go build --ldflags "-w -s -X 'github.com/coreos/etcd-operator/version.GitSHA=$REVISION'" -o /usr/local/bin/etcd-backup-operator github.com/coreos/etcd-operator/cmd/backup-operator
RUN go build --ldflags "-w -s -X 'github.com/coreos/etcd-operator/version.GitSHA=$REVISION'" -o /usr/local/bin/etcd-restore-operator github.com/coreos/etcd-operator/cmd/restore-operator

FROM alpine AS env-builder

# Install SSL ca certificates.
# Ca-certificates is required to call HTTPS endpoints.
RUN apk update && apk add --no-cache ca-certificates && update-ca-certificates

ENV USER=etcd-operator
ENV UID=1000
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/srv/etcd-operator" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

# Use a distroless base image, we don't need anything else as we compiled statically
FROM scratch

# Setup environment with certificates and user
COPY --from=env-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=env-builder /etc/passwd /etc/passwd
COPY --from=env-builder /etc/group /etc/group

COPY --from=builder /usr/local/bin/etcd-operator /usr/local/bin/etcd-operator
COPY --from=builder /usr/local/bin/etcd-backup-operator /usr/local/bin/etcd-backup-operator
COPY --from=builder /usr/local/bin/etcd-restore-operator /usr/local/bin/etcd-restore-operator

COPY --from=builder /tmp /tmp

USER etcd-operator:etcd-operator

ENTRYPOINT ["operator"]
