# etcd Dashboard

## Developing

If you'd like to contribute to the etcd dashboard mod, follow these instructions. For contributing to the rest of etcd, see the contributing document in the root of the repository.

### Install Dependencies

Requires nodejs.  

Run all commands from within the `/mod/dashboard` directory.  

run `./setup` to install npm modules and bower front-end dependencies.

To run a non-compiled development version of the dashboard:  

Continually compile html templates, sass/css, and run unit tests.

```
grunt dev
```

Export an environment varible to notify etcd of the dashboard source code location:  

```
export ETCD_DASHBOARD_DIR=./mod/dashboard/app
```

Run local etc as usual (be sure to include the cors flag).  

```
// from etcd root dir  
./bin/etcd -cors="*"  
```

Alternatively, build the optimized production-build version of the website and run etcd as above:  

```
grunt  
export ETCD_DASHBOARD_DIR=./mod/dashboard/dist  
```
