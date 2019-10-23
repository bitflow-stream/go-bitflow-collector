FROM teambitflow/golang-build:1.12-stretch as build

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
COPY . .
RUN env CC=aarch64-linux-gnu-gcc \
        CGO_ENABLED=1 \
        GOOS=linux \
        GOARCH=arm64 \
        CGO_LDFLAGS="-L/tmp/libpcap-${LIBPCAP_VERSION}" \
        go build -tags "nolibvirt" -o /bitflow-collector ./bitflow-collector

FROM arm64v8/debian:buster-slim
COPY --from=build /bitflow-collector /
ENTRYPOINT ["/bitflow-collector"]