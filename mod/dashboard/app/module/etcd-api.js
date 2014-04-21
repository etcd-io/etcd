/**
 * @fileoverview
 *
 */

'use strict';

angular.module('etcd.module')
.factory('etcdApiSvc', function($http, $q, $, _, pathSvc) {

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
    return $http.get(pathSvc.getStatFullKeyPath(name), {
      supressNotifications: true
    });
  }

  function getPeerUri(peerName) {
    return fetchNode('/_etcd/machines/' + peerName)
    .then(function(peerInfo) {
      var data = decodeURIComponent(peerInfo.value);
      data = data.replace(/&/g, '\",\"').replace(/=/g,'\":\"');
      data = JSON.parse('{"' + data + '"}');
      return data.etcd;
    });
  }

  function getLeaderUri() {
    return fetchLeaderStats()
    .then(function(stats) {
      return getPeerUri(stats.leaderName);
    });
  }

  function fetchPeerDetailStats(peerName) {
    return getPeerUri(peerName).then(function(peerUri) {
      return $http.get(peerUri + pathSvc.getStatFullKeyPath('self'))
      .then(function(resp) {
        return resp.data;
      });
    });
  }

  function fetchLeaderStats() {
    return fetchStat('leader').then(function(resp) {
      var result = {
        followers: [],
        leaderName: resp.data.leader
      };
      _.each(resp.data.followers, function(value, key) {
        value.name = key;
        result.followers.push(value);
      });
      return result;
    });
  }

  return {
    fetch: fetchNode,

    fetchStat: fetchStat,

    fetchLeaderStats: fetchLeaderStats,

    fetchPeerDetailStats: fetchPeerDetailStats,

    getLeaderUri: getLeaderUri,

    create: createNode,

    save: saveNode,

    delete: deleteNode
  };

});
