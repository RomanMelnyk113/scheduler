# Scheduler service

Ensures that the orders are processed by the Kitchen service and that the packages are being delivered by the Drone delivery service.

## Usage
Build the project first
```sh
go build
```
Then make sure food services are running

```sh
./food server --store-namespace=foo --tls-cert=tls.crt --tls-key=tls.key --gcp-project-id=myproject --debug --controller-interval=60s
```
And then finally run
``` sh 
./scheduler -retry-interval=10s -host=127.0.0.1 -port=9000 
```
