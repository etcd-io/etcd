# example of file and environmental variable based configuration of etcd client library

This directory contains tiny examples of file and environmental variable based configuration of the etcd client library.

## How to build

Just execute

```
./build.sh
```

in this directory. `example` is the example program. It just puts or gets 10 keys.

## Launch a cluster for the examples

Just execute

```
./start_cluster.sh
```

in this directory. The script will launch a cluster with `goreman`.

## How to play with `example`

`example` puts 10 keys to the etcd cluster if it is just invoked without any option. If `--get` is passed, `example` gets and checks the keys created by itself.

`env.sh` has an example of configuring the client with environment variables. `file.sh` has an example of configuring the client with yaml file (the config file is `example.yaml` in this directory).
