'use strict';

angular.module('etcdControlPanel')
.controller('BrowserCtrl', function ($scope, $window, EtcdV2, keyPrefix, $, _, moment) {
  $scope.save = 'etcd-save-hide';
  $scope.preview = 'etcd-preview-hide';
  $scope.enableBack = true;
  $scope.writingNew = false;
  $scope.key = null;
  $scope.list = [];

  // etcdPath is the path to the key that is currenly being looked at.
  $scope.etcdPath = keyPrefix;
  $scope.inputPath = keyPrefix;

  $scope.resetInputPath = function() {
    $scope.inputPath = $scope.etcdPath;
  };

  $scope.setActiveKey = function(key) {
    $scope.etcdPath = keyPrefix + _.str.trim(key, '/');
    $scope.resetInputPath();
  };

  $scope.stripPrefix = function(path) {
    return _.str.strRight(path, keyPrefix);
  };

  $scope.onEnter = function() {
    var path = $scope.stripPrefix($scope.inputPath);
    if (path !== '') {
      $scope.setActiveKey(path);
    }
  };

  $scope.updateCurrentKey = function() {
    function etcdPathKey() {
      return pathKey($scope.etcdPath);
    }

    function pathKey(path) {
      var parts = path.split(keyPrefix);
      if (parts.length === 1) {
        return '';
      }
      return parts[1];
    }
    // Notify everyone of the update
    localStorage.setItem('etcdPath', $scope.etcdPath);
    $scope.enableBack = true;
    //disable back button if at root (/v2/keys/)
    if ($scope.etcdPath === keyPrefix) {
      $scope.enableBack = false;
    }
    $scope.key = EtcdV2.getKey(etcdPathKey($scope.etcdPath));
  };

  $scope.$watch('etcdPath', $scope.updateCurrentKey);

  $scope.$watch('key', function() {
    if ($scope.writingNew === true) {
      return;
    }
    $scope.key.get().success(function (data, status, headers, config) {
      //hide any errors
      $('#etcd-browse-error').hide();
      // Looking at a directory if we got an array
      if (data.dir === true) {
        $scope.list = data.node.nodes;
        $scope.preview = 'etcd-preview-hide';
      } else {
        $scope.singleValue = data.node.value;
        $scope.preview = 'etcd-preview-reveal';
        $scope.key.getParent().get().success(function(data) {
          $scope.list = data.node.nodes;
        });
      }
      $scope.previewMessage = 'No key selected.';
    }).error(function (data, status, headers, config) {
      $scope.previewMessage = 'Key does not exist.';
      $scope.showBrowseError(data.message);
    });
  });

  //back button click
  $scope.back = function() {
    $scope.etcdPath = $scope.key.getParent().path();
    $scope.resetInputPath();
    $scope.preview = 'etcd-preview-hide';
    $scope.writingNew = false;
  };

  $scope.showSave = function() {
    $scope.save = 'etcd-save-reveal';
  };

  $scope.saveData = function() {
    $scope.setActiveKey($scope.stripPrefix($scope.inputPath));
    $scope.updateCurrentKey();
    // TODO: fixup etcd to allow for empty values
    $scope.key.set($scope.singleValue || ' ').then(function(response) {
      $scope.save = 'etcd-save-hide';
      $scope.preview = 'etcd-preview-hide';
      $scope.back();
      $scope.writingNew = false;
    }, function (response) {
      $scope.showSaveError(response.message);
    });
  };

  $scope.deleteKey = function(key) {
    $scope.setActiveKey(key);
    $scope.updateCurrentKey();
    $scope.key.deleteKey().then(function(response) {
      //TODO: remove loader
      $scope.save = 'etcd-save-hide';
      $scope.preview = 'etcd-preview-hide';
      $scope.back();
    }, function (response) {
      //TODO: remove loader
      //show errors
      $scope.showBrowseError('Could not delete the key');
    });
  };

  $scope.add = function() {
    $scope.save = 'etcd-save-reveal';
    $scope.preview = 'etcd-preview-reveal';
    $scope.singleValue = '';
    $('.etcd-browser-path').find('input').focus();
    $scope.writingNew = true;
  };

  $scope.showBrowseError = function(message) {
    $('#etcd-browse-error').find('.etcd-popover-content').text('Error: ' + message);
    $('#etcd-browse-error').addClass('etcd-popover-right').show();
  };

  $scope.showSaveError = function(message) {
    $('#etcd-save-error').find('.etcd-popover-content').text('Error: ' + message);
    $('#etcd-save-error').addClass('etcd-popover-left').show();
  };

  $scope.getHeight = function() {
    return $($window).height();
  };

  //$scope.$watch($scope.getHeight, function() {
    ////$('.etcd-container.etcd-browser etcd-body').css('height', $scope.getHeight()-45);
  //});

  $window.onresize = function(){
    $scope.$apply();
  };

});
