'use strict';


angular.module('etcdControlPanel')
.controller('StatsCtrl', function ($scope, $rootScope, $interval, EtcdV2, statsVega, vg) {
  $scope.graphContainer = '#latency';
  $scope.graphVisibility = 'etcd-graph-show';
  $scope.tableVisibility = 'etcd-table-hide';

  //make requests
  function readStats() {
    EtcdV2.getStat('leader').get().success(function(data) {
      $scope.leaderStats = data;
      $scope.leaderName = data.leader;
      $scope.peers = [];
      //hardcode leader stats
      $scope.peers.push({
        latency: {
          average: 0,
          current: 0,
          minimum: 0,
          maximum: 0,
          standardDeviation: 0
        },
        name: data.leader
      });
      $.each(data.followers, function(index, value) {
        value.name = index;
        $scope.peers.push(value);
      });
      //sort array so peers don't jump when output
      $scope.peers.sort(function(a, b){
          if(a.name < b.name) {
            return -1;
          }
          if(a.name > b.name) {
            return 1;
          }
          return 0;
        });
      drawGraph();
    });
  }

  function drawGraph () {
    //hardcoded padding from chart json
    var vertPadding = 30;
    var horzPadding = 15;
    //fetch width and height of graph area
    var width = $($scope.graphContainer).width() - horzPadding;
    var height = $($scope.graphContainer).height() - vertPadding;

    // parse a spec and create a visualization view
    function parse(spec) {
      vg.parse.spec(spec, function(chart) {
        chart({
          el: $scope.graphContainer,
          data: {
            'stats': $scope.peers
          }
        }).width(width).height(height).update();
      });
    }
    parse(statsVega);
  }

  $scope.showTable = function() {
    $scope.tableVisibility = 'etcd-table-reveal';
  };

  $scope.showGraph = function() {
    $scope.tableVisibility = 'etcd-table-hide';
  };

  $scope.getHeight = function() {
    return $(window).height();
  };
  $scope.getWidth = function() {
    return $(window).width();
  };
  //$scope.$watch($scope.getHeight, function() {
    ////$('.etcd-container.etcd-stats .etcd-body').css('height', $scope.getHeight()-5);
    //readStats();
  //});
  $scope.$watch($scope.getWidth, function() {
    readStats();
  });
  window.onresize = function(){
    $scope.$apply();
  };

  $scope.pollPromise = null;

  $scope.startPolling = function() {
    // Update the graphs live
    if ($scope.pollPromise) {
      return;
    }
    $scope.pollPromise = $interval(function() {
      readStats();
    }, 500);
  };

  $scope.stopPolling = function() {
    $interval.cancel($scope.pollPromise);
    $scope.pollPromise = null;
  };

  // Stop polling when navigating away from a view with this controller.
  $rootScope.$on('$routeChangeStart', function () {
    $scope.stopPolling();
  });

  $scope.startPolling();

})


/* statsVega returns the vega configuration for the stats dashboard */
.factory('statsVega', function () {
  return {
    'padding': {'top': 10, 'left': 5, 'bottom': 40, 'right': 10},
    'data': [
      {
        'name': 'stats'
      },
      {
        'name': 'thresholds',
        'values': [50, 100]
      }
    ],
    'scales': [
      {
        'name': 'y',
        'type': 'ordinal',
        'range': 'height',
        'domain': {'data': 'stats', 'field': 'index'}
      },
      {
        'name': 'x',
        'range': 'width',
        'domainMin': 0,
        'domainMax': 100,
        'nice': true,
        'zero': true,
        'domain': {'data': 'stats', 'field': 'data.latency.current'}
      },
      {
        'name': 'color',
        'type': 'linear',
        'domain': [10, 50, 100, 1000000000],
        'range': ['#00DB24', '#FFC000', '#c40022', '#c40022']
      }
    ],
    'axes': [
      {
        'type': 'x',
        'scale': 'x',
        'ticks': 6,
        'name': 'Latency (ms)'
      },
      {
        'type': 'y',
        'scale': 'y',
        'properties': {
          'ticks': {
            'stroke': {'value': 'transparent'}
          },
          'majorTicks': {
            'stroke': {'value': 'transparent'}
          },
          'labels': {
            'fill': {'value': 'transparent'}
          },
          'axis': {
            'stroke': {'value': '#333'},
            'strokeWidth': {'value': 1}
          }
        }
      }
    ],
    'marks': [
      {
        'type': 'rect',
        'from': {'data': 'stats'},
        'properties': {
          'enter': {
            'x': {'scale': 'x', 'value': 0},
            'x2': {'scale': 'x', 'field': 'data.latency.current'},
            'y': {'scale': 'y', 'field': 'index', 'offset': -1},
            'height': {'value': 3},
            'fill': {'scale':'color', 'field':'data.latency.current'}
          }
        }
      },
      {
          'type': 'symbol',
          'from': {'data': 'stats'},
          'properties': {
            'enter': {
              'x': {'scale': 'x', 'field': 'data.latency.current'},
              'y': {'scale': 'y', 'field': 'index'},
              'size': {'value': 50},
              'fill': {'value': '#000'}
            }
          }
        }
      ]
    };
});
