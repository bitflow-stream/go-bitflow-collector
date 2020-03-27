# teambitflow/bitflow-collector:latest-arm32v7
# Copies pre-built binaries into the container. The binaries are built on the local machine beforehand:
# ./arm32v7-build.sh
# docker build -t teambitflow/bitflow-collector:latest-arm32v7 -f arm32v7-prebuilt.Dockerfile _output/arm32v7
FROM arm32v7/debian:buster-slim
RUN ln -s /lib/arm-linux-gnueabihf/ld-linux.so.3 /lib/ld-linux.so.3
COPY bitflow-collector /
COPY bitflow-collector-plugins /bitflow-collector-plugins
COPY run-collector-with-plugins.sh /
ENTRYPOINT ["/run-collector-with-plugins.sh", "-root", ""]
