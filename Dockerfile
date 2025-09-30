FROM ubuntu:24.04

ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=Asia/Shanghai

# 安装locale支持包（根据基础镜像选择）
RUN if [ -f /etc/alpine-release ]; then \
        apk add --no-cache musl-locales; \
    elif [ -f /etc/debian_version ]; then \
        apt-get update && apt-get install -y locales; \
    fi

# 生成en_US.UTF-8 locale
RUN if [ -f /etc/debian_version ]; then \
        localedef -i en_US -f UTF-8 en_US.UTF-8; \
    fi

# 设置环境变量
ENV LANG=en_US.UTF-8 \
    LC_ALL=en_US.UTF-8

RUN apt-get update && \
    apt-get install -y poppler-utils wv unrtf tidy && \
    apt-get install -y inotify-tools && \
    apt-get install -y ca-certificates mailcap curl && \
    apt-get install -y rsync

RUN apt-get install -y p7zip-full && 7z --help

#RUN apt-get install -y flatpak && \
#    flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo && \
#    flatpak install flathub io.github.peazip.PeaZip -y

RUN apt-get install -y unrar && unrar

#RUN apt-get install -y rar && rar

#RUN apt-get install -y wget

#      apt-get install -y sudo make && \
#      wget https://www.rarlab.com/rar/rarlinux-x64-712.tar.gz && \
#      tar -xzvf rarlinux-x64-712.tar.gz &&  \
#      cd rar && \
#      make && \
#      sudo make install && \
#      cd ..; \

RUN if [ "$(uname -m)" = "x86_64" ]; then \
        apt-get install -y rar && rar; \
    elif [ "$(uname -m)" = "aarch64" ]; then \
        echo "There is no rar compressor for arm64"; \
    else \
        echo "Unsupported CPU architecture" && exit 1; \
    fi


#apt-get install -y sudo make gcc g++ \
#      && wget https://www.rarlab.com/rar/rarlinux-x64-712.tar.gz \
#      && tar -xzvf rarlinux-x64-712.tar.gz \
#      && cd rar \
#      && sudo make install \
#      && cd .. \
#      && rar; \

#apt-get install -y sudo \
#        && wget https://www.rarlab.com/rar/rarmacos-arm-712.tar.gz \
#        && tar -xzvf rarmacos-arm-712.tar.gz \
#        && cd rar \
#        && sudo cp rar /usr/local/bin/ \
#        && sudo chmod +x /usr/local/bin/rar \
#        && chmod +x /usr/local/bin/rar \
#        && /usr/local/bin/rar; \

#RUN rar && \
#    unrar

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
