FROM stackbrew/ubuntu:raring

RUN apt-get update && apt-get install -y collectd
RUN adduser --system --group --no-create-home collectd
ADD collectd.conf /etc/collectd/collectd.conf.tmpl
ADD collectd-wrapper /bin/collectd-wrapper
RUN chown -R collectd:collectd /etc/collectd

CMD ["collectd-wrapper"]
