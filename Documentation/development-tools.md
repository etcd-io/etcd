# Development tools

## Vagrant

For fast start you can use Vagrant. `vagrant up` will make etcd build and running on virtual machine. Required Vagrant version is 1.5.0.

Next lets set a single key and then retrieve it:

```
curl -L http://127.0.0.1:4001/v2/keys/mykey -XPUT -d value="this is awesome"
curl -L http://127.0.0.1:4001/v2/keys/mykey
```
