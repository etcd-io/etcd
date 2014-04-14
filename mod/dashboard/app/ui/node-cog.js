'use strict';

angular.module('etcd.ui')
.directive('edNodeCog', function($modal, $rootScope, etcdApiSvc, toastSvc,
      ETCD_EVENT) {

  return {
    templateUrl: '/ui/node-cog.html',
    restrict: 'E',
    replace: true,
    scope: {
      'node': '='
    },
    controller: function($scope){

      function getDeleteMsg() {
        if ($scope.node.dir) {
          return 'Are you sure you want to delete the directory "' +
            $scope.node.key + '" and all of its keys?';
        }
        return 'Are you sure you ant to delete "' + $scope.node.key + '"';
      }

      // Options for both values and directories.
      $scope.cogDropdownOptions = [
        {
          label: 'View Details...',
          callback: function() {
            $modal.open({
              templateUrl: '/page/browser/node-info.html',
              controller: 'NodeInfoCtrl',
              resolve: {
                node: d3.functor($scope.node)
              }
            });
          },
          weight: 100
        },
        {
          label: 'Delete Node...',
          callback: function() {
            $modal.open({
              templateUrl: '/coreos.ui/confirm-modal/confirm-modal.html',
              controller: 'ConfirmModalCtrl',
              resolve: {
                title: d3.functor('Delete Node'),
                message: getDeleteMsg,
                btnText: d3.functor('Delete'),
                errorFormatter: d3.functor('etcdApi'),
                executeFn: _.identity.bind(null, function() {
                  return etcdApiSvc.delete($scope.node)
                  .then(function() {
                    $rootScope.$broadcast(ETCD_EVENT.NODE_DELETED, $scope.node);
                  });
                })
              }
            });
          },
          weight: 300
        }
      ];

      // Options for directories only.
      if ($scope.node.dir) {
        $scope.cogDropdownOptions.push({
          label: 'Modify TTL...',
          callback: function() {
            $modal.open({
              templateUrl: '/page/browser/edit-ttl.html',
              controller: 'EditTtlCtrl',
              resolve: {
                node: d3.functor($scope.node),
              }
            });
          },
          weight: 200
        });
      } else {
        // options for values only.
        $scope.cogDropdownOptions.push({
          label: 'Modify Node...',
          callback: function() {
            $modal.open({
              templateUrl: '/page/browser/edit-node.html',
              controller: 'EditNodeCtrl',
              resolve: {
                node: d3.functor($scope.node),
              }
            });
          },
          weight: 200
        });
      }

    }
  };

});
