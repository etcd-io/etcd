/**
 * @fileoverview
 *
 */

'use strict';

angular.module('etcd.ui')
.directive('coBreadcrumb', function(_, pathSvc) {

  return {
    templateUrl: '/ui/breadcrumb.html',
    restrict: 'E',
    replace: true,
    scope: {
      path: '=',
      callback: '&'
    },
    link: function postLink(scope, elem, attrs) {

      scope.goToRoot = function() {
        scope.callback({ path: '/' });
      };

      scope.onClick = function(part) {
        var selected, selectedPath;
        selected = scope.pathParts.slice(0, scope.pathParts.indexOf(part) + 1);
        selectedPath = pathSvc.make(_.pluck(selected, 'name'));
        scope.callback({ path: selectedPath });
      };

      scope.$watch('path', function(path) {
        scope.pathParts = pathSvc.explode(path).map(function(part) {
          return {
            name: part
          };
        });
      });

    }
  };

});
