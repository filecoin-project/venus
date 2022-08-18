# build container stage
FROM golang:1.17 AS build-env

RUN  sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list

# download dependence
RUN apt-get update -y 
RUN apt-get install -y \
     mesa-opencl-icd ocl-icd-opencl-dev bzr jq pkg-config  hwloc libhwloc-dev gcc  clang build-essential  make ncftp git curl  wget

ADD docker/script/ /script/
RUN bash /script/install_mod.sh
