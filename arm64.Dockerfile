#FROM arm32v7/debian:buster
#FROM balenalib/rpi-debian:buster-run
#COPY ld-linux.so.3 /lib/ld-linux.so.3
FROM arm32v7/debian:buster-slim
COPY qemu-arm-static /usr/bin
RUN ln -s /lib/arm-linux-gnueabihf/ld-linux.so.3 /lib/ld-linux.so.3
COPY bf-with /
ENTRYPOINT ["/bf-with"]

