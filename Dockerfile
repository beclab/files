FROM ubuntu:24.04

#RUN apt-get update && \
#    apt-get install -y poppler-utils wv unrtf tidy && \
#    apt-get install -y inotify-tools && \
#    apt-get install -y ca-certificates mailcap curl && \
#    apt-get install -y rsync

RUN apt-get update && \
    apt-get install -y software-properties-common && \
    add-apt-repository -y universe && \
    sed -i 's|http://archive.ubuntu.com/ubuntu|http://mirrors.aliyun.com/ubuntu|g' /etc/apt/sources.list && \
    apt-get update

RUN apt-get install -y poppler-utils inotify-tools ca-certificates mailcap curl rsync

RUN apt-get install -y wv unrtf tidy

RUN which pdftoppm && which wvtext && which unrtf

HEALTHCHECK --start-period=2s --interval=5s --timeout=3s \
  CMD curl -f http://localhost/health || exit 1

VOLUME /srv
EXPOSE 8080

RUN mkdir dist
COPY cmd/backend/dist dist

# Detect the CPU architecture and copy the appropriate binary
RUN if [ "$(uname -m)" = "x86_64" ]; then \
        cp dist/linux-amd64/filebrowser /filebrowser; \
    elif [ "$(uname -m)" = "aarch64" ]; then \
        cp dist/linux-arm64/filebrowser /filebrowser; \
    else \
        echo "Unsupported CPU architecture" && exit 1; \
    fi

ENTRYPOINT ["/filebrowser"]
