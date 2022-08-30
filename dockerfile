FROM filvenus/venus-buildenv AS buildenv

COPY ./go.mod ./venus/go.mod
COPY ./extern/ ./venus/extern/
RUN export GOPROXY=https://goproxy.cn,direct && cd venus   && go mod download 

COPY . ./venus
RUN export GOPROXY=https://goproxy.cn,direct && cd venus  && make
RUN cd venus && ldd ./venus


FROM filvenus/venus-runtime

# copy the app from build env
COPY --from=buildenv  /go/venus/venus /app/venus
COPY ./docker/script  /script


EXPOSE 3453
ENTRYPOINT ["/app/venus","daemon"]
