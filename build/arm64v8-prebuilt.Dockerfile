# bitflowstream/bitflow-collector:latest-arm64v8
# Copies pre-built binaries into the container. The binaries are built on the local machine beforehand:
# ./containerized-build.sh arm64v8 /tmp/go-mod-cache
# docker build -t bitflowstream/bitflow-collector:latest-arm64v8 -f arm64v8-prebuilt.Dockerfile _output/arm64v8
FROM arm64v8/debian:buster-slim
COPY bitflow-collector /
COPY bitflow-collector-plugins /bitflow-collector-plugins
COPY run-collector-with-plugins.sh /
ENTRYPOINT ["/run-collector-with-plugins.sh", "-root", ""]

# TODO In Jenkinsfile, pcap-support is deactivated for arm64v8 due to a linker error, should be debugged
