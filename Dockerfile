FROM golang:1.11.2-stretch AS builder
MAINTAINER Filecoin Dev Team

RUN apt-get update && apt-get install -y ca-certificates file sudo clang
RUN curl -sSf https://sh.rustup.rs | sh -s -- -y

# This docker file is a modified version of
# https://github.com/ipfs/go-ipfs/blob/master/Dockerfile
# Thanks Lars :)

ENV SRC_DIR /go/src/github.com/filecoin-project/go-filecoin

COPY . $SRC_DIR

# Build the thing.
RUN cd $SRC_DIR \
&& . $HOME/.cargo/env \
&& go run ./build/*go deps \
&& go run ./build/*go build \
&& go build -o ./faucet ./tools/faucet/main.go \
&& go build -o ./genesis-file-server ./tools/genesis-file-server/main.go

# Build gengen
RUN cd

# Get su-exec, a very minimal tool for dropping privileges,
# and tini, a very minimal init daemon for containers
ENV SUEXEC_VERSION v0.2
ENV TINI_VERSION v0.16.1
RUN set -x \
&& cd /tmp \
&& git clone https://github.com/ncopa/su-exec.git \
&& cd su-exec \
&& git checkout -q $SUEXEC_VERSION \
&& make \
&& cd /tmp \
&& wget -q -O tini https://github.com/krallin/tini/releases/download/$TINI_VERSION/tini \
&& chmod +x tini

# need jq for parsing genesis output
RUN cd /tmp \
&& wget -q -O jq https://github.com/stedolan/jq/releases/download/jq-1.5/jq-linux64 \
&& chmod +x jq

# Now comes the actual target image, which aims to be as small as possible.
FROM busybox:1-glibc AS filecoin
MAINTAINER Filecoin Dev Team

# Get the filecoin binary, entrypoint script, and TLS CAs from the build container.
ENV SRC_DIR /go/src/github.com/filecoin-project/go-filecoin
COPY --from=builder $SRC_DIR/go-filecoin /usr/local/bin/go-filecoin
COPY --from=builder $SRC_DIR/bin/container_daemon /usr/local/bin/start_filecoin
COPY --from=builder $SRC_DIR/bin/devnet_start /usr/local/bin/devnet_start
COPY --from=builder $SRC_DIR/bin/node_restart /usr/local/bin/node_restart
COPY --from=builder $SRC_DIR/gengen/gengen /usr/local/bin/gengen
COPY --from=builder $SRC_DIR/fixtures/* /data/
COPY --from=builder /tmp/su-exec/su-exec /sbin/su-exec
COPY --from=builder /tmp/tini /sbin/tini
COPY --from=builder /tmp/filecoin-proof-parameters/* /tmp/filecoin-proof-parameters/
COPY --from=builder /tmp/jq /usr/local/bin/jq
COPY --from=builder /etc/ssl/certs /etc/ssl/certs

# This shared lib (part of glibc) doesn't seem to be included with busybox.
COPY --from=builder /lib/x86_64-linux-gnu/libdl-2.24.so /lib/libdl.so.2
COPY --from=builder /lib/x86_64-linux-gnu/librt.so.1 /lib/librt.so.1
COPY --from=builder /lib/x86_64-linux-gnu/libgcc_s.so.1 /lib/libgcc_s.so.1
COPY --from=builder /lib/x86_64-linux-gnu/libutil.so.1 /lib/libutil.so.1

# Ports for Swarm and CmdAPI
EXPOSE 6000
EXPOSE 3453

# Create the fs-repo directory and switch to a non-privileged user.
ENV FILECOIN_PATH /data/filecoin
RUN mkdir -p $FILECOIN_PATH \
&& adduser -D -h $FILECOIN_PATH -u 1000 -G users filecoin \
&& chown filecoin:users $FILECOIN_PATH

# There is only one bootstrapped miner
RUN cat /data/gen.json | jq ".Miners[0].Address" > /data/minerAddr0 \
&& cat /data/gen.json | jq ".Keys[0]" > /data/walletKey0

# Expose the fs-repo as a volume.
# start_filecoin initializes an fs-repo if none is mounted.
# Important this happens after the USER directive so permission are correct.
VOLUME $FILECOIN_PATH

# There's an fs-repo, and initializes one if there isn't.
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/start_filecoin"]

# Execute the daemon subcommand by default
CMD ["daemon"]
