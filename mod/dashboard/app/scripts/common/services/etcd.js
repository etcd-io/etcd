'use strict';

angular.module('etcd', [])

.factory('EtcdV2', ['$http', '$q', function($http, $q) {
  var keyPrefix = '/v2/keys/'
  var statsPrefix = '/v2/stats/'
  var baseURL = '/v2/'
  var leaderURL = ''

  delete $http.defaults.headers.common['X-Requested-With'];

  function cleanupPath(path) {
    var parts = path.split('/');
    if (parts.length === 0) {
      return '';
    }
    parts = parts.filter(function(v){return v!=='';});
    parts = parts.join('/');
    return parts
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
      var path = '/' + cleanupPath(keyPrefix + self.name);
      if (path === keyPrefix.substring(0, keyPrefix.length - 1)) {
        return keyPrefix
      }
      return path
    };

    self.get = function() {
      return $http.get(self.path());
    };

    self.set = function(keyValue) {
      return getLeader().then(function(leader) {
        return $http({
          url: leader + self.path(),
          data: $.param({value: keyValue}),
          method: 'PUT',
          headers: {'Content-Type': 'application/x-www-form-urlencoded'}
        });
      });
    };

    self.deleteKey = function(keyValue) {
      return getLeader().then(function(leader) {
        return $http({
          url: leader + self.path(),
          method: 'DELETE',
          headers: {'Content-Type': 'application/x-www-form-urlencoded'}
        });
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

  function getLeader() {
    return newStat('leader').get().then(function(response) {
      return newKey('/_etcd/machines/' + response.data.leader).get().then(function(response) {
        // TODO: do something better here p.s. I hate javascript
        var data = decodeURIComponent(response.data.node.value);
        data = data.replace(/&/g, "\",\"").replace(/=/g,"\":\"");
        data = JSON.parse('{"' + data + '"}');
        return data.etcd;
      });
    });
  }

  return {
    getStat: newStat,
    getKey: newKey,
  }
}]);
