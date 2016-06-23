'use strict';

var dashboardApp = angular.module('dashboardApp', ['ngAnimate']);

const eventRowTmpl = `
<div class="jumbotron" ng-model="event">
    <div class="jumbotron-contents">
        <div class="row">
            <div class="col-sm-1 log-img">
                <i class="fa fa-scissors fa-2x" ng-if="event.Code == 1"></i>
                <i class="fa fa-exchange fa-2x" ng-if="event.Code == 2"></i>
                <i class="fa fa-refresh fa-spin fa-2x fa-fw" ng-if="event.Code == 3"></i>
            </div>
            <div class="col-md-10 log-msg">

                <!-- split message -->
                <div ng-if="event.Code == 1">
                    Split
                    <span class="label label-success">Region{{ event.SplitEvent.Region  }}</span> into
                    <span class="label label-success">Region{{ event.SplitEvent.NewRegionA }}</span> and
                    <span class="label label-success">Region{{ event.SplitEvent.NewRegionB }}</span>
                </div>

                <!-- leader transfer message -->
                <div ng-if="event.Code == 2">
                    Transfer leadership of
                    <span class="label label-success">Region{{ event.LeaderTransferEvent.Region }}</span> from 
                    <b>Node{{ event.LeaderTransferEvent.NodeFrom }}</b> to <b> Node{{ event.LeaderTransferEvent.NodeTo }}</b>
                </div>

                <!-- add replica message -->
                <div ng-if="event.Code == 3">
                    Add Replica for <span class="label label-success"> Region{{ event.AddReplicaEvent.Region }} </span>
                </div>

            </div>
        </div>
    </div>
</div>
`;

dashboardApp.directive('eventRow', function() {
    return {
        restrict: 'AE',
        scope: {
            event: '=',
        },
        replace: 'true',
        template: eventRowTmpl
    };
});


dashboardApp.controller('LogEventController', function LogEventController($scope, $timeout) {

    $scope.logs = [];

    $scope.init = function(wsHost) {
            var ws = new WebSocket("ws://" + wsHost + "/ws");

            ws.onopen = function(evt) {
            }

            ws.onclose = function(evt) {
                ws = null;
            }

            ws.onmessage = function(evt) {
                $scope.$apply(function () {
                    var data = JSON.parse(evt.data);
                    $scope.logs.unshift(data);
                });
            }

            ws.onerror = function(evt) {
            }
    };

});
