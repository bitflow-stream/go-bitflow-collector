# teambitflow/bitflow-collector
# Copies pre-built binaries into the container. The binaries are built on the local machine beforehand:
# ./native-build.sh
# docker build -t teambitflow/bitflow-collector -f native-prebuilt.Dockerfile .

# WARNING: this build method is not recommended, as the native libc implementation likely differs from the alpine version

FROM alpine:3.9
RUN apk --no-cache add libvirt-dev libpcap-dev libstdc++
RUN ln -fs libpcap.so /usr/lib/libpcap.so.0.8
COPY _output/native/bitflow-collector /
COPY _output/native/bitflow-collector-plugins /bitflow-collector-plugins
COPY run-collector-with-plugins.sh /
ENTRYPOINT ["/run-collector-with-plugins.sh", "-root", ""]
