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
<<<<<<< HEAD
=======

| Client| [go-etcd](https://github.com/coreos/go-etcd) | [jetcd](https://github.com/diwakergupta/jetcd) | [python-etcd](https://github.com/jplana/python-etcd) | [python-etcd-client](https://github.com/dsoprea/PythonEtcdClient) | [node-etcd](https://github.com/stianeikeland/node-etcd) | [nodejs-etcd](https://github.com/lavagetto/nodejs-etcd) | [etcd-ruby](https://github.com/ranjib/etcd-ruby) | [etcd-api](https://github.com/jdarcy/etcd-api) | [cetcd](https://github.com/dwwoelfel/cetcd) |  [clj-etcd](https://github.com/rthomas/clj-etcd) | [etcetera](https://github.com/drusellers/etcetera)| [Etcd.jl](https://github.com/forio/Etcd.jl) | [p5-etcd](https://metacpan.org/release/Etcd) | [etcdcpp](https://github.com/edwardcapriolo/etcdcpp)  | [etcd-clojure](https://github.com/aterreno/etcd-clojure)
| --- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | 
| **HTTPS Auth**    | Y | Y | Y | Y | Y | Y | - | - | - | - | - | - | - | - | - |
| **Reconnect**     | Y | - | Y | Y | - | - | - | Y | - | - | - | - | - | - | - |
| **Mod/Lock**      | - | - | Y | Y | - | - | - | - | - | - | - | Y | - | - | - |
| **Mod/Leader**    | - | - | - | Y | - | - | - | - | - | - | - | Y | - | - | - |
| **GET Features**  | F | B | F | F | F | F | F | B | F | G | F | F | F | F | F |
| **PUT Features**  | F | B | F | F | F | F | F | G | F | G | F | F | F | F | F |
| **POST Features** | F | - | F | F | - | F | F | - | - | - | F | F | F | G | F |
| **DEL Features**  | F | B | F | F | F | F | F | B | G | B | F | F | F | - | F |

>>>>>>> master
**Legend**
**F**: Full support **G**: Good support **B**: Basic support
**Y**: Feature supported  **-**: Feature not supported

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
|[etcd-clojure](https://github.com/aterreno/etcd-clojure)         |Clojure|-|-|F|F|F|F|-|-|
|[etcetera](https://github.com/drusellers/etcetera)               |.net   |-|-|F|F|F|F|-|-|
|[Etcd.jl](https://github.com/forio/Etcd.jl)                      |Julia  |-|-|F|F|F|F|Y|Y|
|[p5-etcd](https://metacpan.org/release/Etcd)                     |perl   |-|-|F|F|F|F|-|-|
|[etcdcpp](https://github.com/edwardcapriolo/etcdcpp)             |C++    |-|-|F|F|G|-|-|-|

## v1-only clients

Clients supporting only the API version 1

- [justinsb/jetcd](https://github.com/justinsb/jetcd) Java
- [transitorykris/etcd-py](https://github.com/transitorykris/etcd-py) Python
- [russellhaering/txetcd](https://github.com/russellhaering/txetcd) Python
- [iconara/etcd-rb](https://github.com/iconara/etcd-rb) Ruby
- [jpfuentes2/etcd-ruby](https://github.com/jpfuentes2/etcd-ruby) Ruby
- [marshall-lee/etcd.erl](https://github.com/marshall-lee/etcd.erl) Erlang
