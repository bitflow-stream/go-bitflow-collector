# teambitflow/bitflow-collector-build:build-alpine
# This image is used to build the collector for the alpine image. The purpose of this separate container
# is to mount the Go mod-cache into the container during the build, which is not possible with the 'docker build' command.
# See alpine-prebuilt.Dockerfile for further instructions.
# docker build -f alpine-build.Dockerfile -t teambitflow/bitflow-collector-build:alpine .
FROM golang:1.12-alpine
RUN apk --no-cache add git mercurial gcc g++ libvirt-dev libvirt-common-drivers libpcap-dev
WORKDIR /build