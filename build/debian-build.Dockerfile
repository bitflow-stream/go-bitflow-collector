# bitflowstream/golang-collector-build:debian
# docker build -t bitflowstream/golang-collector-build:debian -f debian-build.Dockerfile .
FROM bitflowstream/golang-build:debian
RUN apt install -y libvirt-dev libpcap-dev
