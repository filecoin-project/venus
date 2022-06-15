FROM ubuntu:20.04

# install dependence
RUN apt-get -qq update \
    && apt-get -qq install -y --no-install-recommends ca-certificates curl vim telnet

# set time zone to Shanghai
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
RUN echo 'Asia/Shanghai' >/etc/timezone

# set charset
ENV LANG C.UTF-8
