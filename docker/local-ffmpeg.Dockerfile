#################################################################
FROM --platform=linux/amd64 scratch AS binaries

ADD binaries/mediamtx_*_linux_amd64.tar.gz /linux/amd64
ADD binaries/mediamtx_*_linux_armv6.tar.gz /linux/arm/v6
ADD binaries/mediamtx_*_linux_armv7.tar.gz /linux/arm/v7
ADD binaries/mediamtx_*_linux_arm64.tar.gz /linux/arm64

#################################################################
FROM alpine:3.23

RUN apk add --no-cache ffmpeg \
	&& mkdir -p /recordings \
	&& chown -R 1000:1000 /recordings

ARG TARGETPLATFORM
COPY --from=binaries /$TARGETPLATFORM /

WORKDIR /
USER 1000:1000

ENTRYPOINT [ "/mediamtx" ]
