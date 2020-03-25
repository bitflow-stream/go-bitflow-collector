# teambitflow/bitflow-collector:latest-arm32v7
# Builds the entire collector an all plugins from scratch inside the container.
# Build from the parent directory:
# docker build -t teambitflow/bitflow-collector:latest-arm32v7 -f build/arm32v7-full.Dockerfile .
FROM teambitflow/golang-build:1.12-stretch as build

ENV CC=arm-linux-gnueabi-gcc
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=arm
ENV CGO_LDFLAGS="-L/tmp/libpcap-1.9.0"
ENV LIBPCAP_VERSION=1.9.0

RUN apt-get update && apt-get install -y \
        gcc-arm-linux-gnueabi \
        flex \
        bison \
        byacc \
        libpcap-dev

RUN cd /tmp && \
    wget http://www.tcpdump.org/release/libpcap-${LIBPCAP_VERSION}.tar.gz && \
    tar xvf libpcap-${LIBPCAP_VERSION}.tar.gz && \
    export CC=arm-linux-gnueabi-gcc && \
    cd libpcap-${LIBPCAP_VERSION} && \
    ./configure --host=arm-linux --with-pcap=linux && \
    make

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
RUN go build -tags "nolibvirt" -o /bitflow-collector ./bitflow-collector

# Build the plugins
RUN ./plugins/build-plugins.sh

FROM arm32v7/debian:buster-slim
RUN ln -s /lib/arm-linux-gnueabihf/ld-linux.so.3 /lib/ld-linux.so.3
COPY --from=build /bitflow-collector /
COPY --from=build /build/plugins/_output /bitflow-collector-plugins
COPY build/run-collector-with-plugins.sh /
ENTRYPOINT ["/run-collector-with-plugins.sh"]
