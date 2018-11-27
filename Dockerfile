# hub.docker.com/r/antongulenko/bitflow-collector
FROM golang:alpine as build
RUN apk --no-cache add libvirt-dev libpcap-dev gcc libc-dev
RUN mkdir -p /go/src/github.com/antongulenko/go-bitflow-collector
COPY . /go/src/github.com/antongulenko/go-bitflow-collector/
RUN go get github.com/antongulenko/go-bitflow-collector/bitflow-collector

FROM alpine
RUN apk --no-cache add libvirt-dev libpcap-dev
WORKDIR /root/
COPY --from=build /go/bitflow-collector .
ENTRYPOINT ["./bitflow-collector"]
