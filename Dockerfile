FROM ubuntu:12.04
# Let's install go just like Docker (from source).
RUN apt-get update -q
RUN apt-get install -qy build-essential curl git
RUN curl -s https://go.googlecode.com/files/go1.2.src.tar.gz | tar -v -C /usr/local -xz
RUN cd /usr/local/go/src && ./make.bash --no-clean 2>&1
ENV PATH /usr/local/go/bin:$PATH
ADD . /opt/etcd
RUN cd /opt/etcd && ./build
EXPOSE 4001 7001
ENTRYPOINT ["/opt/etcd/bin/etcd"]

