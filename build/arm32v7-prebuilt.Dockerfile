# bitflowstream/bitflow-collector:latest-arm32v7
# Copies pre-built binaries into the container. The binaries are built on the local machine beforehand:
# ./containerized-build.sh arm32v7 /tmp/go-mod-cache
# docker build -t bitflowstream/bitflow-collector:latest-arm32v7 -f arm32v7-prebuilt.Dockerfile _output/arm32v7
FROM arm32v7/debian:buster-slim

# TODO for some reason, this lib is at the wrong location...
RUN ln -s /lib/arm-linux-gnueabihf/ld-linux.so.3 /lib/ld-linux.so.3

COPY bitflow-collector /
COPY bitflow-collector-plugins /bitflow-collector-plugins
COPY run-collector-with-plugins.sh /

# TODO Loading the plugins does not work ("cannot open shared object file: No such file or directory"), needs to be debugged
# ENTRYPOINT ["/run-collector-with-plugins.sh", "-root", ""]
ENTRYPOINT ["/bitflow-collector"]
