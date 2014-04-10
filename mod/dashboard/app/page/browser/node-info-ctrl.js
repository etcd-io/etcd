'use strict';

angular.module('etcd.page')
.controller('NodeInfoCtrl', function($scope, $modalInstance, _, node) {

  $scope.node = node;

  $scope.objectKeys = _.without(_.keys(node), 'value', '$$hashKey');

  $scope.identityFn = _.identity;

  $scope.close = function() {
    $modalInstance.dismiss('close');
  };

});
