FROM ubuntu:12.04
RUN apt-get update
RUN apt-get install -y python-software-properties git
RUN add-apt-repository -y ppa:duh/golang
RUN apt-get update
RUN apt-get install -y golang
ADD . /opt/etcd
RUN cd /opt/etcd && ./build
RUN ln -s /opt/etcd/etcd /usr/local/bin/etcd
EXPOSE 4001 7001
CMD ["/opt/etcd/etcd", "-c", "0.0.0.0:4001", "-s", "0.0.0.0:7001"]
