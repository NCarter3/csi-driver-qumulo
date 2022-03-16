@echo off

set ver=master
if NOT "%~1"=="" set ver=%1

set repo=https://raw.githubusercontent.com/ScottUrban/csi-driver-qumulo/%ver%/deploy
if NOT "%~2"=="" goto :checkrepo

:checkrepo
    if "%~2"=="local" echo use local deploy
    if "%~2"=="local" set repo=./

if %ver% NEQ master set repo=%repo%/%ver%

echo Uninstalling Qumulo CSI driver, version: %ver%, repo: %repo% ...
kubectl delete -f %repo%/rbac-csi-qumulo-controller.yaml --ignore-not-found
kubectl delete -f %repo%/csi-qumulo-driverinfo.yaml --ignore-not-found
kubectl delete -f %repo%/csi-qumulo-controller.yaml --ignore-not-found
kubectl delete -f %repo%/csi-qumulo-node.yaml --ignore-not-found
echo Uninstalled Qumulo driver successfully.