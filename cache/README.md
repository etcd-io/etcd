# etcd cache

Experimental etcd client cache library.

**Note:** gRPC proxy is not supported. The cache relies on `RequestProgress` RPCs, which the gRPC proxy does not forward.
