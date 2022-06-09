# build container stage
FROM golang:1.16.5 AS build-env

RUN  sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list


# 下载相关依赖
RUN apt-get update -y 
RUN apt-get install -y \
     mesa-opencl-icd ocl-icd-opencl-dev bzr jq pkg-config  hwloc libhwloc-dev
RUN apt-get install -y \
     gcc  clang build-essential  
RUN apt-get install -y \
     make ncftp git curl  wget
