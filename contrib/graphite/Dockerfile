from	stackbrew/ubuntu:precise

run	echo 'deb http://us.archive.ubuntu.com/ubuntu/ precise universe' >> /etc/apt/sources.list
run	apt-get -y update

# Install required packages
run	apt-get -y install python-cairo python-django python-twisted python-django-tagging python-simplejson python-pysqlite2 python-support python-pip gunicorn supervisor nginx-light
run	pip install whisper
run	pip install --install-option="--prefix=/var/lib/graphite" --install-option="--install-lib=/var/lib/graphite/lib" carbon
run	pip install --install-option="--prefix=/var/lib/graphite" --install-option="--install-lib=/var/lib/graphite/webapp" graphite-web

# Add system service config
add	./nginx.conf /etc/nginx/nginx.conf
add	./supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Add graphite config
add	./initial_data.json /var/lib/graphite/webapp/graphite/initial_data.json
add	./local_settings.py /var/lib/graphite/webapp/graphite/local_settings.py
add	./carbon.conf /var/lib/graphite/conf/carbon.conf
add	./storage-schemas.conf /var/lib/graphite/conf/storage-schemas.conf
run	mkdir -p /var/lib/graphite/storage/whisper
run	touch /var/lib/graphite/storage/graphite.db /var/lib/graphite/storage/index
run	chown -R www-data /var/lib/graphite/storage
run	chmod 0775 /var/lib/graphite/storage /var/lib/graphite/storage/whisper
run	chmod 0664 /var/lib/graphite/storage/graphite.db
run cd /var/lib/graphite/webapp/graphite && python manage.py syncdb --noinput

expose	:80
expose	:2003

cmd	["/usr/bin/supervisord"]
