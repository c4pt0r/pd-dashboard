'use strict';

var dashboardApp = angular.module('dashboardApp', []);

dashboardApp.controller('LogEventController', function LogEventController($scope) {
    $scope.events = [1,2,3]; 
});
