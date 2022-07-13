FROM ubuntu:20.04

# install dependence
RUN apt-get -qq update \
    && apt-get -qq install -y --no-install-recommends ca-certificates curl vim telnet tzdata 

# set time zone to Shanghai
ENV TZ=Asia/Shanghai
RUN ln -fs /usr/share/zoneinfo/${TZ} /etc/localtime \
    && echo ${TZ} > /etc/timezone \
    && dpkg-reconfigure --frontend noninteractive tzdata


# set charset
ENV LANG C.UTF-8
