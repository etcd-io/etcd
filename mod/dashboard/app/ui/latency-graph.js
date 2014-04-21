'use strict';

angular.module('etcd.ui').directive('coLatencyGraph', function(etcdApiSvc,
      $, d3, _, graphConfig) {

  return {
    templateUrl: '/ui/latency-graph.html',
    restrict: 'E',
    replace: true,
    scope: {
      peerData: '='
    },
    link: function postLink(scope, elem, attrs) {
      var padding = 60;

      function updateGraph() {
        var width = elem.width() - padding,
            height = 300;

        vg.parse.spec(graphConfig, function(chart) {
          chart({
            el: elem[0],
            data: {
              'stats': scope.peerData
            }
          })
          .width(width)
          .height(height)
          .update();
        });
      }

      scope.$watch('peerData', function(peerData) {
        if (!_.isEmpty(peerData)) {
          updateGraph();
        }
      });

      elem.on('$destroy', function() {
      });

    }
  };

});
