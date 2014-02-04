'use strict';

angular.module('etcdControlPanel')
.factory('prefixUrl', function(urlPrefix) {

  return function(url) {
    return urlPrefix + url;
  }

});
