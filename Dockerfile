#FROM alpine:latest
#RUN apk --update add ca-certificates \
#                     mailcap \
#                     curl

FROM ubuntu:22.04
RUN apt-get update && \
    apt-get install -y poppler-utils wv unrtf tidy && \
    apt-get install -y inotify-tools && \
    apt-get install -y ca-certificates mailcap curl

HEALTHCHECK --start-period=2s --interval=5s --timeout=3s \
  CMD curl -f http://localhost/health || exit 1

VOLUME /srv
EXPOSE 8110

COPY packages/backend/docker_config.json /.filebrowser.json
COPY packages/backend/filebrowser /filebrowser

ENTRYPOINT [ "/filebrowser" ]