'use strict';

angular.module('etcdControlPanel')
.directive('highlight', function(keyPrefix) {
  return {
    restrict: 'E',
    scope: {
      highlightBase: '=',
      highlightCurrent: '='
    },
    link: function(scope, element, attrs) {
      var base = _.str.strRight(scope.highlightBase, keyPrefix),
          current = _.str.trim(scope.highlightCurrent, '/');
      if (base === current) {
        element.parent().parent().addClass('etcd-selected');
      }
    }
  };
});
