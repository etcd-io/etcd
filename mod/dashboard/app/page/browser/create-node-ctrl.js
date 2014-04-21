'use strict';

angular.module('etcd.page')
.controller('CreateNodeCtrl', function($scope, $rootScope, $modalInstance, _,
      ETCD_EVENT, etcdApiSvc, pathSvc, key) {

  $scope.key = key;
  if (key === '/') {
    $scope.keyInputPrefix = '/';
  } else {
    $scope.keyInputPrefix = pathSvc.truncate(key, 20) + '/';
  }

  $scope.save = function(node) {
    $scope.requestPromise = etcdApiSvc.create(node)
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
angular.module('etcd.page').controller('CreateNodeFormCtrl', function($scope, pathSvc) {

  $scope.fields = {
    key: '',
    value: '',
    type: 'key',
    ttl: null
  };

  $scope.submit = function() {
    var newNode = {};
    newNode.key = pathSvc.clean($scope.key + '/' + $scope.fields.key);
    newNode.dir = $scope.fields.type === 'dir';
    newNode.ttl = $scope.fields.ttl ? parseInt($scope.fields.ttl, 10) : null;
    if (!newNode.dir) {
      newNode.value = $scope.fields.value;
    }
    $scope.save(newNode);
  };

});
