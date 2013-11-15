FROM ubuntu:12.04
RUN apt-get update
RUN apt-get install -y python-software-properties git
RUN add-apt-repository -y ppa:duh/golang
RUN apt-get update
RUN apt-get install -y golang
ADD . /opt/etcd
RUN cd /opt/etcd && ./build
EXPOSE 4001 7001
ENTRYPOINT ["/opt/etcd/etcd", "-addr", "0.0.0.0:4001", "-bind-addr", "0.0.0.0:7001"]
