# teambitflow/bitflow-collector:latest-arm64v8
# Copies pre-built binaries into the container. The binaries are built on the local machine beforehand:
# ./arm64v8-build.sh
# docker build -t teambitflow/bitflow-collector:latest-arm64v8 -f arm64v8-prebuilt.Dockerfile .
FROM arm64v8/debian:buster-slim
RUN ln -s /lib/arm-linux-gnueabihf/ld-linux.so.3 /lib/ld-linux.so.3
COPY _output/arm64v8/bitflow-collector /
COPY _output/arm64v8/bitflow-collector-plugins /bitflow-collector-plugins
COPY run-collector-with-plugins.sh /
ENTRYPOINT ["/run-collector-with-plugins.sh", "-root", ""]
