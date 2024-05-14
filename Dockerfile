FROM amazonlinux:2023

RUN yum install -y make iproute tar gzip gcc procps protobuf-compiler && \
    curl -L -o go.tar.gz https://go.dev/dl/go1.22.3.linux-arm64.tar.gz && \
    tar zxvf go.tar.gz 

ENV PATH="${PATH}:${PWD}/go/bin"

WORKDIR teleport

COPY . .

# Cache modules
RUN go mod download -x

RUN make && cp build/* /bin/
