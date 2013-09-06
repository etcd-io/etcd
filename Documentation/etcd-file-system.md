#Etcd File System

## Structure
[TODO]
![alt text](./img/etcd_fs_structure.jpg "etcd file system structure")

## Node
In **Etcd**, the **Node** is the rudimentary element constructing the whole.
Currently **Etcd** file system is comprised in a Unix-like way of files and directories, and they are two kinds of nodes different in:

- **File Node** has data associated with it.
- **Directory Node** has children nodes associated with it.

Besides the file and directory difference, all nodes have common attributes and operations as follows:

### Attributes:
- **Expiration Time** [optional]

  The node will be deleted when it expires.

- **ACL**

  The path of access control list of the node.

### Operation:
- **Get** (path, recursive)

  Get the content of the node
    - If the node is a file, the data of the file will be returned.
    - If the node is a directory, the child nodes of the directory will be returned.
    - If recursive is true, it will recursively get the nodes of the directory.

- **Set** (path, value[optional], ttl [optional])

  Set the value to a file. Set operation will help to create intermediate directories with no expiration time.
    - If the value is given, set will create a file
    - If the value is not given, set will crate a directory
    - If ttl is given, the node will be deleted when it expires.

- **Delete** (path, recursive)

  Delete the node of given path.
    - If the node is a directory:
    - If recursive is true, the operation will delete all nodes under the directory.
    - If recursive is false, error will be returned.

- **TestAndSet** (path, prevValue [prevIndex], value, ttl)

  Atomic *test and set* value to a file. If test succeeds, this operation will change the previous value of the file to the given value.
    - If the prevValue is given, it will test against previous value of 
    the node.
    - If the prevValue is empty, it will test if the node is not existing.
    - If the prevValue is not empty, it will test if the prevValue is equal to the current value of the file.
    - If the prevIndex is given, it will test if the create/last modified index of the node is equal to prevIndex.

- **Renew** (path, ttl)

  Set the node's expiration time to (current time + ttl)

## ACL
[TODO]

## User Group
[TODO]
