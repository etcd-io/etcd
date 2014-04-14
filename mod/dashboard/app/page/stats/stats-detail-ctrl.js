'use strict';

angular.module('etcd.page')
.controller('StatsDetailCtrl', function($scope, $modalInstance, _, etcdApiSvc,
      peerName) {

  etcdApiSvc.fetchPeerDetailStats(peerName)
  .then(function(stats) {
    $scope.stats = stats;
    $scope.objectKeys = _.without(_.keys($scope.stats), '$$hashKey');
  });

  $scope.identityFn = _.identity;

  $scope.close = function() {
    $modalInstance.dismiss('close');
  };

});
