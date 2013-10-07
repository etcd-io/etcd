'use strict';

angular.module('etcd', [])

.factory('EtcdV1', ['$http', function($http) {
  var keyPrefix = '/v1/keys/'
  var statsPrefix = '/v1/stats/'
  var baseURL = '/v1/'

  delete $http.defaults.headers.common['X-Requested-With'];

  function cleanupPath(path) {
    var parts = path.split('/');
    if (parts.length === 0) {
      return '';
    }
    parts = parts.filter(function(v){return v!=='';});
    return parts.join('/');
  }

  function newKey(keyName) {
    var self = {};
    self.name = cleanupPath(keyName);

    self.getParent = function() {
      var parts = self.name.split('/');
      if (parts.length === 0) {
        return newKey('');
      }
      parts.pop();
      return newKey(parts.join('/'));
    };

    self.path = function() {
      return '/' + cleanupPath(keyPrefix + self.name);
    };

    self.get = function() {
      return $http.get(self.path());
    };

    self.set = function(keyValue) {
      return $http({
        url: self.path(),
        data: $.param({value: keyValue}),
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'}
      });
    };

    self.deleteKey = function(keyValue) {
      return $http({
        url: self.path(),
        method: 'DELETE',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'}
      });
    };

    return self;
  }

  function newStat(statName) {
    var self = {};
    self.name = cleanupPath(statName);

    self.path = function() {
      return '/' + cleanupPath(statsPrefix + self.name);
    };

    self.get = function() {
      return $http.get(self.path());
    };

    return self
  }

  return {
    getStat: newStat,
    getKey: newKey
  }
}]);
