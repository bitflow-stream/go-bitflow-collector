# bitflowstream/bitflow-collector:latest-arm32v7
# Copies pre-built binaries into the container. The binaries are built on the local machine beforehand:
# ./containerized-build.sh arm32v7 /tmp/go-mod-cache
# docker build -t bitflowstream/bitflow-collector:latest-arm32v7 -f arm32v7-prebuilt.Dockerfile _output/arm32v7
FROM arm32v7/debian:buster-slim
COPY bitflow-collector /
COPY bitflow-collector-plugins /bitflow-collector-plugins
COPY run-collector-with-plugins.sh /
ENTRYPOINT ["/run-collector-with-plugins.sh", "-root", ""]
