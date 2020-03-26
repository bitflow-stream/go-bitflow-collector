# teambitflow/bitflow-collector-build:arm64v8
# This image is used to build the collector for the ARM processor. The purpose of this separate container
# is to mount the Go mod-cache into the container during the build, which is not possible with the 'docker build' command.
# See arm64v8-prebuilt.Dockerfile for further instructions.
# docker build -f arm64v8-build.Dockerfile -t teambitflow/bitflow-collector-build:arm64v8 .
FROM teambitflow/golang-build:1.12-stretch

ENV CC=aarch64-linux-gnu-gcc
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=arm64
ENV CGO_LDFLAGS="-L/tmp/libpcap-1.9.0"
ENV LIBPCAP_VERSION=1.9.0

RUN apt-get update && apt-get install -y \
        gcc-aarch64-linux-gnu \
        flex \
        bison \
        byacc \
        libpcap-dev

RUN cd /tmp && \
    wget http://www.tcpdump.org/release/libpcap-${LIBPCAP_VERSION}.tar.gz && \
    tar xvf libpcap-${LIBPCAP_VERSION}.tar.gz && \
    export CC=aarch64-linux-gnu-gcc && \
    cd libpcap-${LIBPCAP_VERSION} && \
    ./configure --host=arm-linux --with-pcap=linux && \
    make

WORKDIR /build
