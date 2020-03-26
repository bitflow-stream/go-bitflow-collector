# teambitflow/bitflow-collector-build:arm32v7
# This image is used to build the collector for the ARM processor. The purpose of this separate container
# is to mount the Go mod-cache into the container during the build, which is not possible with the 'docker build' command.
# See arm32v7-prebuilt.Dockerfile for further instructions.
# docker build -f arm32v7-build.Dockerfile -t teambitflow/bitflow-collector-build:arm32v7 .
FROM teambitflow/golang-build:1.12-stretch

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
