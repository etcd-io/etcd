# Use goreman to run `go install github.com/mattn/goreman@latest`
# Change the path of bin/etcd if etcd is located elsewhere

etcd1: bin/etcd --name infra1 --listen-client-urls http://127.0.0.1:2379 --advertise-client-urls http://127.0.0.1:2379 --listen-peer-urls http://127.0.0.1:12380 --initial-advertise-peer-urls http://127.0.0.1:12380 --initial-cluster-token etcd-cluster-1 --initial-cluster 'infra1=http://127.0.0.1:12380,infra2=http://127.0.0.1:22380,infra3=http://127.0.0.1:32380' --initial-cluster-state new --enable-pprof --logger=zap --log-outputs=stderr
etcd2: bin/etcd --name infra2 --listen-client-urls http://127.0.0.1:22379 --advertise-client-urls http://127.0.0.1:22379 --listen-peer-urls http://127.0.0.1:22380 --initial-advertise-peer-urls http://127.0.0.1:22380 --initial-cluster-token etcd-cluster-1 --initial-cluster 'infra1=http://127.0.0.1:12380,infra2=http://127.0.0.1:22380,infra3=http://127.0.0.1:32380' --initial-cluster-state new --enable-pprof --logger=zap --log-outputs=stderr
etcd3: bin/etcd --name infra3 --listen-client-urls http://127.0.0.1:32379 --advertise-client-urls http://127.0.0.1:32379 --listen-peer-urls http://127.0.0.1:32380 --initial-advertise-peer-urls http://127.0.0.1:32380 --initial-cluster-token etcd-cluster-1 --initial-cluster 'infra1=http://127.0.0.1:12380,infra2=http://127.0.0.1:22380,infra3=http://127.0.0.1:32380' --initial-cluster-state new --enable-pprof --logger=zap --log-outputs=stderr
#proxy: bin/etcd grpc-proxy start --endpoints=127.0.0.1:2379,127.0.0.1:22379,127.0.0.1:32379 --listen-addr=127.0.0.1:23790 --advertise-client-url=127.0.0.1:23790 --enable-pprof

# A learner node can be started using the below Procfile.learner (uncomment and run)

# Use goreman to run `go install github.com/mattn/goreman@latest`

# 1. Start the cluster using Procfile
# 2. Add learner node to the cluster
#   % etcdctl member add infra4 --peer-urls="http://127.0.0.1:42380" --learner=true

# 3. Start learner node with goreman
# Change the path of bin/etcd if etcd is located elsewhere

# uncomment below to setup

# etcd4: bin/etcd --name infra4 --listen-client-urls http://127.0.0.1:42379 --advertise-client-urls http://127.0.0.1:42379 --listen-peer-urls http://127.0.0.1:42380 --initial-advertise-peer-urls http://127.0.0.1:42380 --initial-cluster-token etcd-cluster-1 --initial-cluster 'infra4=http://127.0.0.1:42380,infra1=http://127.0.0.1:12380,infra2=http://127.0.0.1:22380,infra3=http://127.0.0.1:32380' --initial-cluster-state existing --enable-pprof --logger=zap --log-outputs=stderr

# 4. The learner node can be promoted to voting member by the command
#   % etcdctl member promote <memberid>

