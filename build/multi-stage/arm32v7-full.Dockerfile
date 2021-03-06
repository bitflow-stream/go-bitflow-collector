# bitflowstream/bitflow-collector:latest-arm32v7
# Builds the entire collector and all plugins from scratch inside the container.
# Build from the repository root directory:
# docker build -t bitflowstream/bitflow-collector:latest-arm32v7 -f build/multi-stage/arm32v7-full.Dockerfile .
FROM golang:1.14.1-buster as build
RUN apt-get update && apt-get install -y git mercurial qemu-user gcc-arm-linux-gnueabi
WORKDIR /build
ENV GO111MODULE=on
ENV GOOS=linux
ENV GOARCH=arm
ENV CC=arm-linux-gnueabi-gcc
ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-L/tmp/libpcap-1.9.0"
ENV LIBPCAP_VERSION=1.9.0

RUN apt-get update && apt-get install -y flex bison byacc libpcap-dev

RUN cd /tmp && \
    wget http://www.tcpdump.org/release/libpcap-${LIBPCAP_VERSION}.tar.gz && \
    tar xvf libpcap-${LIBPCAP_VERSION}.tar.gz && \
    export CC=arm-linux-gnueabi-gcc && \
    cd libpcap-${LIBPCAP_VERSION} && \
    ./configure --host=arm-linux --with-pcap=linux && \
    make

# Copy go.mod first and download dependencies, to enable the Docker build cache
COPY go.mod .
RUN sed -i $(find -name go.mod) -e '\_//.*gitignore$_d' -e '\_#.*gitignore$_d'
RUN go mod download

# Copy rest of the source code and build
# Delete go.sum files and clean go.mod files form local 'replace' directives
COPY . .
RUN find -name go.sum -delete
RUN sed -i $(find -name go.mod) -e '\_//.*gitignore$_d' -e '\_#.*gitignore$_d'
RUN ./build/native-build.sh -tags "nolibvirt"

# Build the plugins
RUN ./plugins/build-plugins.sh build/_output/native/bitflow-collector-plugins

# Build the plugins
RUN ./plugins/build-plugins.sh
COPY --from=build /build/build/_output/native/bitflow-collector /
COPY --from=build /build/build/_output/native/bitflow-collector-plugins /bitflow-collector-plugins
COPY --from=build /build/build/run-collector-with-plugins.sh /
ENTRYPOINT ["/run-collector-with-plugins.sh", "-root", ""]
