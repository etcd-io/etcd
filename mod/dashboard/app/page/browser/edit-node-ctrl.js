'use strict';

angular.module('etcd.page')
.controller('EditNodeCtrl', function($scope, $rootScope, $modalInstance, _,
      ETCD_EVENT, etcdApiSvc, pathSvc, node) {

  $scope.node = node;

  $scope.displayKey = pathSvc.truncate(node.key, 50) + '/'

  $scope.save = function(node) {
    $scope.requestPromise = etcdApiSvc.save(node)
    .then(function() {
      $rootScope.$broadcast(ETCD_EVENT.NODE_CHANGED, node);
      $modalInstance.close(node);
    });
  };

  $scope.cancel = function() {
    $modalInstance.dismiss('cancel');
  };

});


/**
 * Controller for the form. Must be different than the modal controller.
 */
angular.module('etcd.page').controller('EditNodeFormCtrl', function($scope, pathSvc) {

  $scope.fields = {
    value: $scope.node.value || '',
    ttl: $scope.node.ttl || ''
  };

  $scope.submit = function() {
    var editNode = {
      key: $scope.node.key,
      value: $scope.fields.value,
      ttl: $scope.fields.ttl ? parseInt($scope.fields.ttl, 10) : null
    };
    $scope.save(editNode);
  };

});
