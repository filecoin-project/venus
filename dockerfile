FROM filvenus/venus-buildenv AS buildenv

COPY . ./venus
RUN export GOPROXY=https://goproxy.cn && cd venus  && make
RUN cd venus && ldd ./venus


FROM filvenus/venus-runtime

# DIR for app
WORKDIR /app

# copy the app from build env
COPY --from=buildenv  /go/venus/venus /app/venus
COPY ./docker/script  /script


EXPOSE 3453
ENTRYPOINT ["/app/venus","daemon"]
