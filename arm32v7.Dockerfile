FROM teambitflow/golang-build:1.12-stretch as build

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
COPY . .
RUN env CC=arm-linux-gnueabi-gcc \
        CGO_ENABLED=1 \
        GOOS=linux \
        GOARCH=arm \
        CGO_LDFLAGS="-L/tmp/libpcap-${LIBPCAP_VERSION}" \
        go build -tags "nolibvirt" -o /bitflow-collector ./bitflow-collector

FROM arm32v7/debian:buster-slim
RUN ln -s /lib/arm-linux-gnueabihf/ld-linux.so.3 /lib/ld-linux.so.3
COPY --from=build /bitflow-collector /
ENTRYPOINT ["/bitflow-collector"]