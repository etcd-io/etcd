'use strict';

angular.module('etcd.page')
.controller('BrowserCtrl', function($scope, $modal, etcdApiSvc, pathSvc,
      ETCD_EVENT, d3, pollerSvc) {

  $scope.currPath = '/';
  $scope.currNode = null;

  $scope.rowClick = function(node) {
    if (node.dir) {
      $scope.currPath = node.key;
    }
  };

  $scope.truncateKey = function(key) {
    return pathSvc.tail(key);
  };

  $scope.openCreateModal = function(key) {
    $modal.open({
      templateUrl: '/page/browser/create-node.html',
      controller: 'CreateNodeCtrl',
      resolve: {
        key: d3.functor(key || $scope.currPath),
      }
    });
  };

  /**
   * Refresh the list whenever a node is changed.
   */
  $scope.$on(ETCD_EVENT.NODE_CHANGED, function(e, node) {
    var parentKey = pathSvc.getParent(node.key);
    $scope.refreshNode(parentKey);
  });

  /**
   * Refresh the list whenever a node is deleted.
   */
  $scope.$on(ETCD_EVENT.NODE_DELETED, function(e, node) {
    var parentKey = pathSvc.getParent(node.key);
    $scope.refreshNode(parentKey);
  });

  $scope.breadcrumbCallback = function(result) {
    $scope.currPath = result.path;
  };

  /**
   * Update currPath and always refetch that node.
   */
  $scope.refreshNode = function(path) {
    if ($scope.currPath === path) {
      // Force refresh.
      $scope.fetchNode();
    } else {
      $scope.currPath = path;
    }
  };

  $scope.fetchNode = function() {
    return etcdApiSvc.fetch($scope.currPath)
    .then(function(node) {
      $scope.currNode = node;
    });
  };

  $scope.$watch('currPath', function(currPath) {
    if (!currPath || currPath === '') {
      $scope.currPath = '/';
      return;
    }
    $scope.fetchNode();
  });

  pollerSvc.register('nodePoller', {
    fn: $scope.fetchNode,
    scope: $scope
  });

});

