FROM golang

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/engine/reference/builder/#copy
COPY . .

# Build
RUN CGO_ENABLED=0 go build -o /usr/bin/network-runtime cmd/server/server.go

ARG CNI_PLUGINS_VERSION="v1.3.0"
ARG CNI_PLUGINS_CLONE_URL="https://github.com/containernetworking/plugins"
RUN git clone --filter=tree:0 "${CNI_PLUGINS_CLONE_URL}" /cni-plugins \
    && cd /cni-plugins \
    && git checkout "${CNI_PLUGINS_VERSION}" \
    && CGO_ENABLED=0 ./build_linux.sh && mkdir -p /opt/cni/bin \
    && cp -a ./bin/. /opt/cni/bin/

RUN apt update && apt install iptables -y

# Run
CMD ["/usr/bin/network-runtime"]