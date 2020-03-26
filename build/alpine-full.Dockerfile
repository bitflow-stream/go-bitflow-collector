# teambitflow/bitflow-collector
# Builds the entire collector an all plugins from scratch inside the container.
# Build from the parent directory:
# docker build -t teambitflow/bitflow-collector -f build/alpine-full.Dockerfile .
FROM golang:1.12-alpine as build
RUN apk --no-cache add git mercurial gcc g++ libvirt-dev libvirt-common-drivers libpcap-dev
WORKDIR /build

# Copy go.mod first and download dependencies, to enable the Docker build cache
COPY go.mod .
RUN sed -i $(find -name go.mod) -e '\_//.*gitignore$_d' -e '\_#.*gitignore$_d'
RUN go mod download

# Copy rest of the source code and build
# Delete go.sum files and clean go.mod files form local 'replace' directives
COPY . .
RUN find -name go.sum -delete
RUN sed -i $(find -name go.mod) -e '\_//.*gitignore$_d' -e '\_#.*gitignore$_d'
RUN go build -o /bitflow-collector ./bitflow-collector

# Build the plugins
RUN ./plugins/build-plugins.sh ./plugins/_output

FROM alpine:3.9
RUN apk --no-cache add libvirt-dev libpcap-dev libstdc++ curl
COPY --from=build /bitflow-collector /
COPY --from=build /build/plugins/_output /bitflow-collector-plugins
COPY build/run-collector-with-plugins.sh /
ENTRYPOINT ["/run-collector-with-plugins.sh"]
