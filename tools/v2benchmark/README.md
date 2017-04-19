# ETCD Load Testing

Note : This test, right now, supports only a single node etcd cluster.

This is a etcd load test module written in go-language. It basically creates 
artificial load on a running etcd instance. The test runs for a random set of
keys and values, which can be specified in the configuration file. You can 
configure many other parameters in the config file. See the sample config file 
*etcd_load.cfg* for more information. 


##Features
 - Memory information
  - gives the memory information about the running etcd instance -- before, 
  	after and difference, when requests are made
  -	To get this info, use the "-mem" flag while running the module
  - Also, if the instance is running on a remote machine, then you need to use
  	the "-remote" flag as well.
  - The memory information if obtained using the following "pmap" command. Basically
  	the RSS part of the total memory usage, is used.
  ```
  	$ pmap -x $(pidof etcd) | tail -n1 | awk '{print $4}'
  ```
  ### Sample Output of : pmap -x $pid_of_etcd
	```	  
	  3084:   etcd -addr 10.70.1.148:4001
	Address           Kbytes     RSS   Dirty Mode  Mapping
	0000000000400000    3020    2764       0 r-x-- etcd
	00000000006f3000    3316    2996       0 r---- etcd
	0000000000a30000     120     116      28 rw--- etcd
	0000000000a4e000     116      96      96 rw---   [ anon ]
	000000c000000000      36      36      36 rw---   [ anon ]
	000000c207de8000   36448   15188   15188 rw---   [ anon ]
	00007fcc92ad1000    1728     660     660 rw---   [ anon ]
	00007ffc139c0000     132      12      12 rw---   [ stack ]
	00007ffc139ed000       8       0       0 r----   [ anon ]
	00007ffc139ef000       8       4       0 r-x--   [ anon ]
	ffffffffff600000       4       0       0 r-x--   [ anon ]
	---------------- ------- ------- ------- 
	total kB           44936   21872   16020
	```

 - Key value distribution
  - This feature basically allows you to specify the distribution of the 
  	key-values, that is how many keys lie in a particular value range, specified
  	by "value-range" parameter under section : "section-args", in etcd_load.cfg
  - See the "pct" parameter under the section :"section-args", in etcd_load.cfg
 - Value range
  - This allows you to specify value ranges, which will be used for the -- Key
  	value distribution feature.
  - Note : length (Value Range) = length (Key Value distribution) + 1
 - There are other configuration options as well, like -- log-file, remote-flag,
 	remote-host-user, etcd. . Some of these can be specified using commandline 
 	flags as well, in which case the flags will override the cfg-file values. To
 	know more about them see the default config file -- "etcd_load.cfg" and for
 	help regarding flags, use
 		- go run etcd_load.go -h

##Setup
 - Make sure that the following packages are in your GOPATH. Set your 
   GOPATH if its not already set.
   In go, to get a "package" you can simply do : go get package. 
   In this use case do it under your GOPATH.
 - Packages :
  - "github.com/coreos/go-etcd/etcd"
  - "code.google.com/p/gcfg"
  - "code.google.com/p/go.crypto/ssh"
 - Example : 
  - go get "code.google.com/p/gcfg"
 - Set up a default config file, like the one available in the repo.
  - Below is a sample config file, for more details take a look at etcd_load.cfg
   
### Sample Config File
```
		[section-args]
		etcdhost="127.0.0.1"
		etcdport="4001"
		operation=create
		keycount="100"
		operation-count="200"
		log-file=log
		threads=5 
		pct="5,74,10,10,1"
		value-range="0,256,512,1024,8192,204800"
		remote-flag=False
		ssh-port=22
		remote-host-user=root
```

##Build
 - Now, to run etcd_load test, use the following steps
```
 $ go build etcd_load.go report.go
```

##Running the Test

 - ./etcd_load -c config_file [flag]
  - flags : -help , -h , -p , -o , -k , -oc , -log , -mem , -remote
  - For details regarding the flags , use :
  	```
    $ go run etcd_load.go -help
	```
 - Examples :

  	[remote etcd instance]
   	```
 	$ ./etcd_load -c etcd_load.cfg -mem -remote -o create  
 	$ ./etcd_load -c etcd_load.cfg -h 10.10.10.1 -p 4001 -o create 
   	```

    [local etcd instance]
    ```
 	$ ./etcd_load -c etcd_load.cfg -h 127.0.0.1 -o create 
	```

	Note that the "-c" flag is compulsory, that is you need to have a default 
	config file that must be input using the -c flag
	To know more about the flags :: do -- go run etcd_load.go -h


##Result 
 - You can find more runtime details in the log file. 
 - The commandline the report looks like :

*******************************************************************
	Summary:
	  Total:	2.8454 secs.
	  Slowest:	0.1493 secs.
	  Fastest:	0.0001 secs.
	  Average:	0.0258 secs.
	  Requests/sec:	35.1449

	Response time histogram:
	  0.000 [1]		|
	  0.015 [17]	|∎∎∎∎∎∎∎∎∎∎∎∎∎∎
	  0.030 [48]	|∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
	  0.045 [27]	|∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
	  0.060 [3]		|∎∎
	  0.075 [2]		|∎
	  0.090 [0]		|
	  0.105 [1]		|
	  0.119 [0]		|
	  0.134 [0]		|
	  0.149 [1]		|

	Latency distribution:
	  10% in 0.0006 secs.
	  25% in 0.0226 secs.
	  50% in 0.0240 secs.
	  75% in 0.0308 secs.
	  90% in 0.0332 secs.
	  95% in 0.0481 secs.
	  99% in 0.1493 secs.


##Credit ::
 - The report.go file in the package is used from the "boom" package, here is the link
 	- https://github.com/rakyll/boom/blob/master/boomer/print.go
