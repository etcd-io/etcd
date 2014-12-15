# Running etcd as a Service

In order to better integrate with the Windows Services model, we can run etcd as a Service.
This will require Administrative user credentials to run the commands.

## Installing the Service

Use `sc create etcd binPath= "C:\etcd\etcd.exe -data-dir=C:\etcd\etcd.service.data"` to
install the `etcd` service.

The service name must same as the `etcd` application's base name (`C:\xxx\etcd.exe`'s base name is `etcd`).
If we use a special service name, we should change the etcd application name.

For example, we can use `myetcd` as service name:

1. `rename etcd.exe myetcd.exe`
2. `sc create myetcd binPath= "C:\etcd\myetcd.exe -data-dir=C:\etcd\myetcd.service.data"`

This command does not start the service.

## Uninstalling the Service

If the service is running, use `net stop etcd` to stop the service first.
Then use `sc delete etcd` to remove the service.

## Starting the Service

`net start etcd` will start the service

## Stopping the Service

`net stop etcd` will stop the service.

