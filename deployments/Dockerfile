FROM --platform=$BUILDPLATFORM docker.io/golang:1.20 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go test -v ./...

WORKDIR /app/cmd/integration-cip-havochvatten

ARG TARGETARCH
ARG TARGETOS
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build

FROM registry.access.redhat.com/ubi9/ubi-minimal
WORKDIR /opt/diwise

RUN /bin/microdnf reinstall tzdata -y
ENV TZ=Europe/Stockholm

COPY --from=builder --chown=1001 /app/cmd/integration-cip-havochvatten/integration-cip-havochvatten /opt/diwise

RUN chown 1001 /opt/diwise

USER 1001

ENTRYPOINT ["/opt/diwise/integration-cip-havochvatten"]
