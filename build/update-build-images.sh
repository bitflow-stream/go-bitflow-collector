#!/bin/bash

# Build images
docker build -t bitflowstream/golang-collector-build:debian -f collector-build-debian.Dockerfile .
docker build -t bitflowstream/golang-collector-build:alpine -f collector-build-alpine.Dockerfile .
docker build -t bitflowstream/golang-collector-build:arm32v7 -f collector-build-arm32v7.Dockerfile .
docker build -t bitflowstream/golang-collector-build:arm64v8 -f collector-build-arm64v8.Dockerfile .

# Push updated images
docker push bitflowstream/golang-collector-build:debian
docker push bitflowstream/golang-collector-build:alpine
docker push bitflowstream/golang-collector-build:arm32v7
docker push bitflowstream/golang-collector-build:arm64v8
