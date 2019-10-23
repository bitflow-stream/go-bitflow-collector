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

#RUN apt-get update && apt-get install -y \
#        libgnutls28-dev \
#        libnl-route-3-dev \
#        libnl-3-dev \
#        libxml2 \
#        libxml2-dev \
#        libxml2-utils \
#        xsltproc \
#        libyajl-dev \
#        systemtap-sdt-dev \
#        libdevmapper-dev \
#        libpciaccess-dev
#
#RUN ln -s /usr/include/x86_64-linux-gnu/sys/sdt.h /usr/aarch64-linux-gnu/include/sys/sdt.h
#RUN ln -s /usr/lib/x86_64-linux-gnu/libyajl.so /usr/aarch64-linux-gnu/lib/libyajl.so


#RUN cd /tmp && \
#    wget https://libvirt.org/sources/libvirt-5.7.0.tar.xz && \
#    tar xvf libvirt-5.7.0.tar.xz && \
#    export CC=aarch64-linux-gnu-gcc && \
#    cd libvirt-5.7.0 && \
#    ./configure --host=arm-linux --with-qemu=yes --with-selinux=no --with-dtrace --disable-nls && \
#    make


WORKDIR /build
COPY . .
RUN env CC=aarch64-linux-gnu-gcc \
        CGO_ENABLED=1 \
        GOOS=linux \
        GOARCH=arm64 \
        CGO_LDFLAGS="-L/tmp/libpcap-${LIBPCAP_VERSION}" \
        go build -tags "nolibvirt" -o /bitflow-collector ./bitflow-collector

FROM arm64v8/debian:buster-slim
#RUN ln -s /lib/arm-linux-gnueabihf/ld-linux.so.3 /lib/ld-linux.so.3
COPY --from=build /bitflow-collector /
ENTRYPOINT ["/bitflow-collector"]