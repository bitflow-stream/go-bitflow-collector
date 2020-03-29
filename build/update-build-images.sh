#!/bin/bash

# Build images
docker build -t teambitflow/bitflow-collector-build:debian -f collector-build-debian.Dockerfile .
docker build -t teambitflow/bitflow-collector-build:alpine -f collector-build-alpine.Dockerfile .
docker build -t teambitflow/bitflow-collector-build:arm32v7 -f collector-build-arm32v7.Dockerfile .
docker build -t teambitflow/bitflow-collector-build:arm64v8 -f collector-build-arm64v8.Dockerfile .

# Push updated images
docker push teambitflow/bitflow-collector-build:debian
docker push teambitflow/bitflow-collector-build:alpine
docker push teambitflow/bitflow-collector-build:arm32v7
docker push teambitflow/bitflow-collector-build:arm64v8
