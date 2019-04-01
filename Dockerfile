# antongulenko/bitflow-collector
FROM golang:1.11-alpine as build
ENV GO111MODULE=on
RUN apk --no-cache add git gcc g++ libvirt-dev libvirt-common-drivers libpcap-dev
WORKDIR /build
COPY . .

# TODO this workaround should not be necessary
RUN rm go.sum

RUN go build -o /bitflow-collector ./bitflow-collector
ENTRYPOINT ["/bitflow-collector"]

FROM alpine
RUN apk --no-cache add libvirt-dev libpcap-dev libstdc++ curl
COPY --from=build /bitflow-collector /
ENTRYPOINT ["/bitflow-collector"]

