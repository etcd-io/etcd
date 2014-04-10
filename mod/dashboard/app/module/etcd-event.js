'use strict';

angular.module('etcd.module').constant('ETCD_EVENT', {
  NODE_DELETED: 'etcd.node_deleted',
  NODE_CHANGED: 'etcd.node_changed'
});
