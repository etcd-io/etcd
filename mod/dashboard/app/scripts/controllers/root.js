'use strict';

angular.module('etcdControlPanel')
.controller('RootCtrl', function($rootScope, prefixUrl) {

  // Expose prefixUrl() function to all.
  $rootScope.prefixUrl = prefixUrl;

});
