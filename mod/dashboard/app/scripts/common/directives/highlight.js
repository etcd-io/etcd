'use strict';

angular.module('etcdControlPanel')
.directive('highlight', function() {
  return {
    restrict: 'A',
    link: function(scope, element, attrs) {
      if('#' + scope.etcdPath === attrs.href) {
        element.parent().parent().addClass('etcd-selected');
      }
    }
  };
});
