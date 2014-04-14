'use strict';

angular.module('etcd.page')
.controller('StatsCtrl', function($scope, $modal, etcdApiSvc, pollerSvc) {

  $scope.followers = null;
  $scope.leader = null;
  $scope.leaderName = null;

  $scope.parseLatencyStats = function(stats) {
    $scope.followers = stats.followers;
    $scope.leaderName = stats.leaderName;
  };

  $scope.fetchLeaderDetails = function() {
    return etcdApiSvc.fetchPeerDetailStats($scope.leaderName)
    .then(function(leaderSelfStats) {
      $scope.leader = {
        name: $scope.leaderName,
        uptime: leaderSelfStats.leaderInfo.uptime
      };
    });
  };

  $scope.$watch('leaderName', function(leaderName) {
    if (leaderName) {
      pollerSvc.kill('leaderDetailsPoller');
      pollerSvc.register('leaderDetailsPoller', {
        fn: $scope.fetchLeaderDetails,
        scope: $scope,
        interval: 5000
      });
    }
  });

  $scope.openDetailModal = function(peerName) {
    $modal.open({
      templateUrl: '/page/stats/stats-detail.html',
      controller: 'StatsDetailCtrl',
      resolve: {
        peerName: d3.functor(peerName),
      }
    });
  };

  $scope.getSquareStatusClass = function(follower) {
    if (follower.latency.current < 25) {
      return 'ed-m-square-status--green';
    }
    if (follower.latency.current < 60) {
      return 'ed-m-square-status--orange';
    }
    return 'ed-m-square-status--red';
  };

  pollerSvc.register('statsPoller', {
    fn: etcdApiSvc.fetchLeaderStats,
    then: $scope.parseLatencyStats,
    scope: $scope,
    interval: 500
  });

});
