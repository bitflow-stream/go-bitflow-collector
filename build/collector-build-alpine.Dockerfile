# teambitflow/bitflow-collector-build:alpine
# This image is used to build the collector for the alpine image. The purpose of this separate container
# is to mount the Go mod-cache into the container during the build, which is not possible with the 'docker build' command.
# See alpine-prebuilt.Dockerfile for further instructions.
# docker build -t teambitflow/bitflow-collector-build:alpine -f collector-build-alpine.Dockerfile .
FROM teambitflow/golang-build:alpine
RUN apk --no-cache add libvirt-dev libvirt-common-drivers libpcap-dev
