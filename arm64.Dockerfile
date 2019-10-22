FROM teambitflow/golang-build:1.12-stretch as build
#FROM ubuntu:18.04 as build
#FROM arm32v7/debian:buster
RUN dpkg --add-architecture armhf
#RUN apt-get update && apt-get install -y libgnutls28-dev:armhf \
#                                         xsltproc:armhf \
#                                         #libxml-xpath-perl:armhf \
#                                         libyajl-dev:armhf \
#                                         libdevmapper-dev:armhf \
#                                         libpciaccess-dev:armhf \
#                                         libnl-route-3-dev:armhf \
#                                         libnl-3-dev:armhf \
#                                         systemtap-sdt-dev:armhf \
#                                         uuid-dev:armhf \
#                                         libtool:armhf \
#                                         autoconf:armhf \
#                                         pkg-config:armhf \
#                                         libxml2:armhf \
#                                         libxml2-dev:armhf \
#                                         libxml2-utils:armhf \
#                                         autopoint:armhf \
#                                         python-dev:armhf \
#                                         #libnuma-dev:armhf \
#                                         gettext:armhf \
#                                         wget:armhf \
#                                         gcc-arm-linux-gnueabi
#
RUN apt-get update && apt-get install -y gcc-arm-linux-gnueabi \
                                         libvirt-dev:armhf
                                         #libvirt-common-drivers:armhf \
                                         #libpcap-dev:armhf

#RUN ln -s /usr/include/x86_64-linux-gnu/sys/sdt.h /usr/arm-linux-gnueabi/include/sys/sdt.h


WORKDIR /build
COPY . .
RUN env CC=arm-linux-gnueabi-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm go build -tags "nopcap" -o /bitflow-collector ./bitflow-collector

#RUN cd /tmp && \
#    wget https://libvirt.org/sources/libvirt-5.7.0.tar.xz && \
#    tar xvf libvirt-5.7.0.tar.xz && \
#    export CC=arm-linux-gnueabi-gcc && \
#    cd libvirt-5.7.0 && \
#    ./configure --host=arm-linux --with-qemu=yes --with-dtrace --disable-nls && \
#    make