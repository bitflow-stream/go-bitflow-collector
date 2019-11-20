# teambitflow/bitflow-collector:latest-arm64v8
FROM teambitflow/golang-build:1.12-stretch as build

ENV CC=arm64-linux-gnueabi-gcc
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

FROM arm64v8/debian:buster-slim
COPY --from=build /bitflow-collector /
ENTRYPOINT ["/bitflow-collector"]
