FROM ubuntu:20.04

# 证书下载
RUN apt-get -qq update \
    && apt-get -qq install -y --no-install-recommends ca-certificates curl vim telnet

# 设置时区为上海
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
RUN echo 'Asia/Shanghai' >/etc/timezone

# 设置编码
ENV LANG C.UTF-8
