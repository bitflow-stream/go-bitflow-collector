# teambitflow/bitflow-collector
FROM golang:1.11-alpine as build
ENV GO111MODULE=on
RUN apk --no-cache add git gcc g++ libvirt-dev libvirt-common-drivers libpcap-dev
WORKDIR /build
COPY . .
RUN rm go.sum # TODO HACK
RUN go build -o /bitflow-collector ./bitflow-collector

FROM alpine
RUN apk --no-cache add libvirt-dev libpcap-dev libstdc++ curl
COPY --from=build /bitflow-collector /
ENTRYPOINT ["/bitflow-collector"]

