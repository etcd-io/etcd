'use strict';

angular.module('etcd.page')
.controller('EditTtlCtrl', function($scope, $rootScope, $modalInstance, _,
      ETCD_EVENT, etcdApiSvc, node) {

  $scope.node = node;

  $scope.save = function(ttl) {
    node.ttl = ttl;
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
angular.module('etcd.page').controller('EditTtlFormCtrl', function($scope) {

  $scope.fields = {
    ttl: $scope.node.ttl || ''
  };

  $scope.submit = function() {
    $scope.save($scope.fields.ttl);
  };

});
