#!/bin/bash

# Build images
docker build -t bitflowstream/golang-collector-build:debian -f debian-build.Dockerfile .
docker build -t bitflowstream/golang-collector-build:alpine -f alpine-build.Dockerfile .
docker build -t bitflowstream/golang-collector-build:arm32v7 -f arm32v7-build.Dockerfile .
docker build -t bitflowstream/golang-collector-build:arm64v8 -f arm64v8-build.Dockerfile .

# Push updated images
docker push bitflowstream/golang-collector-build:debian
docker push bitflowstream/golang-collector-build:alpine
docker push bitflowstream/golang-collector-build:arm32v7
docker push bitflowstream/golang-collector-build:arm64v8
