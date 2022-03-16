@echo off

set ver=master
if NOT "%~1"=="" set ver=%1

set repo=https://raw.githubusercontent.com/ScottUrban/csi-driver-qumulo/%ver%/deploy
if NOT "%~2"=="" goto :checkrepo

:checkrepo
    if "%~2"=="local" echo use local deploy
    if "%~2"=="local" set repo=./

if %ver% NEQ master set repo=%repo%/%ver%

echo Installing Qumulo CSI driver, version: %ver%, repo: %repo% ...
kubectl apply -f %repo%/rbac-csi-qumulo-controller.yaml
kubectl apply -f %repo%/csi-qumulo-driverinfo.yaml
kubectl apply -f %repo%/csi-qumulo-controller.yaml
kubectl apply -f %repo%/csi-qumulo-node.yaml
echo Qumulo CSI driver installed successfully.