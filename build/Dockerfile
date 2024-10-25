ARG BASE_IMAGE=ubuntu:22.04
# Build the manager binary
FROM golang@sha256:f43c6f049f04cbbaeb28f0aad3eea15274a7d0a7899a617d0037aec48d7ab010 AS builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY power-optimization-library/ power-optimization-library/
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY build/manager/main.go main.go
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/
COPY e-sms/ e-sms/

# Build
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

FROM ${BASE_IMAGE}
ARG MANIFEST=build/manifests/power-node-agent-ds.yaml
WORKDIR /
COPY LICENSE /licenses/LICENSE
COPY --from=builder /workspace/manager .
COPY ${MANIFEST} /power-manifests/
COPY e-sms/e_smi/lib/ /usr/local/lib/
USER 10001

ENV LD_LIBRARY_PATH=/usr/local/lib

ENTRYPOINT ["/manager"]
