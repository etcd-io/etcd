

build:
	./build

static.build:
	./build --static

static.image: static.build
	mkdir -p static/bin/ && \
		cp -ap bin/etcd static/bin/etcd && \
		cd static && \
		strip ./bin/etcd && \
		docker build -t etcd-static .

clean:
	rm -rf ./static/bin/ ./bin

