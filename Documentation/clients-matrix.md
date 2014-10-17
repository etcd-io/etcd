# Client libraries support matrix for etcd

As etcd features support is really uneven between client libraries, a compatibility matrix can be important.

## v2 clients

The v2 API has a lot of features, we will categorize them in a few categories:
- **Language**: The language in which the client library was written.
- **HTTPS Auth**: Support for SSL-certificate based authentication
- **Reconnect**: If the client is able to reconnect automatically to another server if one fails.
- **Mod/Lock**: Support for the locking module
- **Mod/Leader**: Support for the leader election module
- **GET,PUT,POST,DEL Features**: Support for all the modifiers when calling the etcd server with said HTTP method.

### Supported features matrix

**Legend**
**F**: Full support **G**: Good support **B**: Basic support
**Y**: Feature supported  **-**: Feature not supported

Sorted alphabetically on language/name

|Client |**Language**|**HTTPS Auth**|**Re-connect**|**GET**|**PUT**|**POST**|**DEL**|**Mod Lock**|**Mod Leader**|
| --- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | 
|[etcd-api](https://github.com/jdarcy/etcd-api)                   |C      |-|Y|B|G|-|B|-|-|
|[etcdcpp](https://github.com/edwardcapriolo/etcdcpp)             |C++    |-|-|F|F|G|-|-|-|
|[cetcd](https://github.com/dwwoelfel/cetcd)                      |Clojure|-|-|F|F|-|G|-|-|
|[clj-etcd](https://github.com/rthomas/clj-etcd)                  |Clojure|-|-|G|G|-|B|-|-|
|[etcd-clojure](https://github.com/aterreno/etcd-clojure)         |Clojure|-|-|F|F|F|F|-|-|
|[go-etcd](https://github.com/coreos/go-etcd)                     |go     |Y|Y|F|F|F|F|-|-|
|[boon etcd client](https://github.com/boonproject/boon/blob/master/etcd/README.md)        |java   |Y|Y|F|F|F|F|-|F|
|[etcd4j](https://github.com/jurmous/etcd4j)                      |java   |Y|Y|F|F|F|F|-|-|
|[jetcd](https://github.com/diwakergupta/jetcd)                   |java   |Y|-|B|B|-|B|-|-|
|[jetcd](https://github.com/justinsb/jetcd)                       |java   |-|-|B|B|-|B|-|-|
|[Etcd.jl](https://github.com/forio/Etcd.jl)                      |Julia  |-|-|F|F|F|F|Y|Y|
|[etcetera](https://github.com/drusellers/etcetera)               |.net   |-|-|F|F|F|F|-|-|
|[node-etcd](https://github.com/stianeikeland/node-etcd)          |nodejs |Y|-|F|F|-|F|-|-|
|[nodejs-etcd](https://github.com/lavagetto/nodejs-etcd)          |nodejs |Y|-|F|F|F|F|-|-|
|[p5-etcd](https://metacpan.org/release/Etcd)                     |perl   |-|-|F|F|F|F|-|-|
|[python-etcd](https://github.com/jplana/python-etcd)             |python |Y|Y|F|F|F|F|Y|-|
|[python-etcd-client](https://github.com/dsoprea/PythonEtcdClient)|python |Y|Y|F|F|F|F|Y|Y|
|[txetcd](https://github.com/russellhaering/txetcd)               |python |-|-|G|G|F|G|-|-|
|[etcd-ruby](https://github.com/ranjib/etcd-ruby)                 |ruby   |-|-|F|F|F|F|-|-|
