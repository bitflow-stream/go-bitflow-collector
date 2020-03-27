# teambitflow/bitflow-collector
# Copies pre-built binaries into the container. The binaries are built on the local machine beforehand:
# ./alpine-build.sh
# docker build -t teambitflow/bitflow-collector -f alpine-prebuilt.Dockerfile _output/alpine
FROM alpine:3.9
RUN apk --no-cache add libvirt-dev libpcap-dev libstdc++
COPY bitflow-collector /
COPY bitflow-collector-plugins /bitflow-collector-plugins
COPY run-collector-with-plugins.sh /
ENTRYPOINT ["/run-collector-with-plugins.sh", "-root", ""]
