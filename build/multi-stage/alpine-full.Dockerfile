# bitflowstream/bitflow-collector
# Builds the entire collector and all plugins from scratch inside the container.
# Build from the repository root directory:
# docker build -t bitflowstream/bitflow-collector -f build/multi-stage/alpine-full.Dockerfile .
FROM golang:1.14.1-alpine as build
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
RUN ./build/native-build.sh

# Build the plugins
RUN ./plugins/build-plugins.sh build/_output/native/bitflow-collector-plugins

FROM alpine:3.11.5
RUN apk --no-cache add libvirt-dev libpcap-dev libstdc++
COPY --from=build /build/build/_output/native/bitflow-collector /
COPY --from=build /build/build/_output/native/bitflow-collector-plugins /bitflow-collector-plugins
COPY --from=build /build/build/run-collector-with-plugins.sh /
ENTRYPOINT ["/run-collector-with-plugins.sh"]
