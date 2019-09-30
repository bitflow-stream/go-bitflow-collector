FROM teambitflow/golang-build:1.12-stretch as build
RUN apt-get update && apt-get install -y gcc-arm-linux-gnueabi flex bison byacc libpcap-dev libvirt-dev
RUN cd /tmp && \
    wget http://www.tcpdump.org/release/libpcap-1.9.0.tar.gz && \
    tar xvf libpcap-1.9.0.tar.gz && \
    export CC=arm-linux-gnueabi-gcc && \
    cd libpcap-1.9.0 && \
    ./configure --host=arm-linux --with-pcap=linux && \
    make

WORKDIR /build
COPY . .
RUN env CC=arm-linux-gnueabi-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm CGO_LDFLAGS="-L/tmp/libpcap-1.9.0" go build -tags "nolibvirt" -o /bitflow-collector ./bitflow-collector

FROM arm32v7/debian:buster-slim
COPY qemu-arm-static /usr/bin
RUN ln -s /lib/arm-linux-gnueabihf/ld-linux.so.3 /lib/ld-linux.so.3
COPY --from=build /bitflow-collector /
ENTRYPOINT ["/bitflow-collector"]