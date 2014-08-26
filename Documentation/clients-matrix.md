# Client libraries support matrix for etcd

As etcd features support is really uneven between client libraries, a compatibility matrix can be important.
We will consider in detail only the features of clients supporting the v2 API. Clients still supporting the v1 API *only* are listed below.

## v2 clients

The v2 API has a lot of features, we will categorize them in a few categories:
- **Language**: The language in which the client library was written.
- **HTTPS Auth**: Support for SSL-certificate based authentication
- **Reconnect**: If the client is able to reconnect automatically to another server if one fails.
- **Mod/Lock**: Support for the locking module
- **Mod/Leader**: Support for the leader election module
- **GET,PUT,POST,DEL Features**: Support for all the modifiers when calling the etcd server with said HTTP method.

### Supported features matrix

|Client |**Language**|**HTTPS Auth**|**Re-connect**|**GET**|**PUT**|**POST**|**DEL**|**Mod Lock**|**Mod Leader**|
| --- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | 
|[go-etcd](https://github.com/coreos/go-etcd)                     |go     |Y|Y|F|F|F|F|-|-|
|[jetcd](https://github.com/diwakergupta/jetcd)                   |java   |Y|-|B|B|-|B|-|-|
|[python-etcd](https://github.com/jplana/python-etcd)             |python |Y|Y|F|F|F|F|Y|-|
|[python-etcd-client](https://github.com/dsoprea/PythonEtcdClient)|python |Y|Y|F|F|F|F|Y|Y|
|[node-etcd](https://github.com/stianeikeland/node-etcd)          |nodejs |Y|-|F|F|-|F|-|-|
|[nodejs-etcd](https://github.com/lavagetto/nodejs-etcd)          |nodejs |Y|-|F|F|F|F|-|-|
|[etcd-ruby](https://github.com/ranjib/etcd-ruby)                 |ruby   |-|-|F|F|F|F|-|-|
|[etcd-api](https://github.com/jdarcy/etcd-api)                   |C      |-|Y|B|G|-|B|-|-|
|[cetcd](https://github.com/dwwoelfel/cetcd)                      |Clojure|-|-|F|F|-|G|-|-|
|[clj-etcd](https://github.com/rthomas/clj-etcd)                  |Clojure|-|-|G|G|-|B|-|-|
|[etcetera](https://github.com/drusellers/etcetera)               |.net   |-|-|F|F|F|F|-|-|
|[Etcd.jl](https://github.com/forio/Etcd.jl)                      |Julia  |-|-|F|F|F|F|Y|Y|
|[p5-etcd](https://metacpan.org/release/Etcd)                     |perl   |-|-|F|F|F|F|-|-|
|[etcdcpp](https://github.com/edwardcapriolo/etcdcpp)             |C++    |-|-|F|F|G|-|-|-|

**Legend**

**F**: Full support **G**: Good support **B**: Basic support
**Y**: Feature supported  **-**: Feature not supported

## v1-only clients

Clients supporting only the API version 1

- [justinsb/jetcd](https://github.com/justinsb/jetcd) Java
- [transitorykris/etcd-py](https://github.com/transitorykris/etcd-py) Python
- [russellhaering/txetcd](https://github.com/russellhaering/txetcd) Python
- [iconara/etcd-rb](https://github.com/iconara/etcd-rb) Ruby
- [jpfuentes2/etcd-ruby](https://github.com/jpfuentes2/etcd-ruby) Ruby
- [aterreno/etcd-clojure](https://github.com/aterreno/etcd-clojure) Clojure
- [marshall-lee/etcd.erl](https://github.com/marshall-lee/etcd.erl) Erlang
