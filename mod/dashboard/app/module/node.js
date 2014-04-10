/**
 * @fileoverview
 *
 */

'use strict';

angular.module('etcd.module')
.factory('nodeSvc', function($http, $q, $, _, pathSvc, toastSvc) {

  function createNode(node) {
    var payload  = {
      ttl: node.ttl
    };
    if (node.dir) {
      payload.dir = true;
    } else {
      payload.value = node.value;
    }
    return getLeaderUri()
    .then(function(leaderUri) {
      return $http({
        url: leaderUri + pathSvc.getFullKeyPath(node.key),
        data: $.param(payload),
        method: 'PUT',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
      });
    });
  }

  function saveNode(node) {
    var payload  = {
      ttl: node.ttl,
      prevExist: true
    };
    if (node.dir) {
      payload.dir = true;
    } else {
      payload.value = node.value;
    }
    return getLeaderUri()
    .then(function(leaderUri) {
      return $http({
        url: leaderUri + pathSvc.getFullKeyPath(node.key),
        data: $.param(payload),
        method: 'PUT',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
      });
    });
  }

  function deleteNode(node) {
    var params;
    if (node.dir) {
      params = {
        dir: true,
        recursive: true
      };
    }
    return getLeaderUri()
    .then(function(leaderUri) {
      return $http({
        url: leaderUri + pathSvc.getFullKeyPath(node.key),
        method: 'DELETE',
        params: params,
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
      });
    });
  }

  function fetchNode(key) {
    return $http.get(pathSvc.getFullKeyPath(key), {
      supressNotifications: true
    })
    .then(function(resp) {
      return resp.data.node;
    });
  }

  function fetchStat(name) {
    return $http.get(pathSvc.getStatFullKeyPath(name));
  }

  function getLeaderUri() {
    return fetchStat('leader')
    .then(function(resp) {
      return fetchNode('/_etcd/machines/' + resp.data.leader)
      .then(function(leaderNode) {
        var data = decodeURIComponent(leaderNode.value);
        data = data.replace(/&/g, '\",\"').replace(/=/g,'\":\"');
        data = JSON.parse('{"' + data + '"}');
        return data.etcd;
      });
    })
    .catch(function() {
      toastSvc.error('Error fetching leader.');
    });
  }

  return {
    fetch: fetchNode,

    fetchStat: fetchStat,

    getLeaderUri: getLeaderUri,

    create: createNode,

    save: saveNode,

    delete: deleteNode
  };

});
