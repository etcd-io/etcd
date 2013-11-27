'use strict';

angular.module('etcdBrowser', ['ngRoute', 'etcd', 'timeRelative'])

.constant('keyPrefix', '/v2/keys/')

.config(['$routeProvider', 'keyPrefix', function ($routeProvider, keyPrefix) {
  //read localstorage
  var previousPath = localStorage.getItem('etcd_path');

  $routeProvider
    .when('/', {
      redirectTo: keyPrefix
    })
    .otherwise({
      templateUrl: 'views/browser.html',
      controller: 'MainCtrl'
    });
}])

.controller('MainCtrl', ['$scope', '$location', 'EtcdV2', 'keyPrefix', function ($scope, $location, EtcdV2, keyPrefix) {
  $scope.save = 'etcd-save-hide';
  $scope.preview = 'etcd-preview-hide';
  $scope.enableBack = true;
  $scope.writingNew = false;

  // etcdPath is the path to the key that is currenly being looked at.
  $scope.etcdPath = $location.path();

  $scope.$watch('etcdPath', function() {
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
    if($scope.etcdPath === keyPrefix) {
      $scope.enableBack = false;
    }

    $scope.key = EtcdV2.getKey(etcdPathKey($scope.etcdPath));
  });

  $scope.$watch('key', function() {
    if ($scope.writingNew === true) {
      return;
    }
    $scope.key.get().success(function (data, status, headers, config) {
      //hide any errors
      $('#etcd-browse-error').hide();
      // Looking at a directory if we got an array
      if (data.dir === true) {
        $scope.list = data.kvs;
        $scope.preview = 'etcd-preview-hide';
      } else {
        $scope.singleValue = data.value;
        $scope.preview = 'etcd-preview-reveal';
        $scope.key.getParent().get().success(function(data) {
          $scope.list = data.kvs;
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
    $scope.syncLocation();
    $scope.preview = 'etcd-preview-hide';
    $scope.writingNew = false;
  };

  $scope.syncLocation = function() {
    $location.path($scope.etcdPath);
  };

  $scope.showSave = function() {
    $scope.save = 'etcd-save-reveal';
  };

  $scope.saveData = function() {
    // TODO: fixup etcd to allow for empty values
    $scope.key.set($scope.singleValue || ' ').then(function(response) {
      $scope.save = 'etcd-save-hide';
      $scope.preview = 'etcd-preview-hide';
      $scope.back();
      $scope.writingNew = false;
    }, function (response) {
      $scope.showSaveError(data.message);
    });
  };

  $scope.deleteKey = function() {
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
    return $(window).height();
  };
  $scope.$watch($scope.getHeight, function() {
    $('.etcd-body').css('height', $scope.getHeight()-45);
  });
  window.onresize = function(){
    $scope.$apply();
  };

}])

.directive('ngEnter', function() {
  return function(scope, element, attrs) {
    element.bind('keydown keypress', function(event) {
      if(event.which === 13) {
        scope.$apply(function(){
          scope.$eval(attrs.ngEnter);
        });

        event.preventDefault();
      }
    });
  };
})

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

moment.lang('en', {
  relativeTime : {
    future: 'Expires in %s',
    past:   'Expired %s ago',
    s:  'seconds',
    m:  'a minute',
    mm: '%d minutes',
    h:  'an hour',
    hh: '%d hours',
    d:  'a day',
    dd: '%d days',
    M:  'a month',
    MM: '%d months',
    y:  'a year',
    yy: '%d years'
  }
});
