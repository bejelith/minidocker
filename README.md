# [RFD](./RFD.md)

# Build
`make build` will create a `build` with all executables

From project directory:
```
$ make 
$ ./build/testrunner -help
```

run tests  
`make test`

**NOTE:** tests don't fully pass on darwin

## Build and test using docker for Mac
Build the image  
`docker build -t test .`  
Run tests  
`docker run -ti --rm --privileged test make test`

# Using testrunner
testrunner utility works fully on linux, I have not invested to make it run on docker for mac.

On docker:  
`docker run -ti --rm --privileged test testrunner ip link`  
**NOTE:** shows system interfaces; as i wrote before i have not investigated why.

# CA creation

All certificates are generate under the `ca` directory by the ssl.go script which is invoked by 
```
make ca
```

# Start the server
For help:  
`server -h`  

With flags:  
`server -ca ca/ca.crt -cert ca/server.crt -key ca/server.key -listen 127.0.0.1:8080`

# Using the client
Invoking help:  
```
./build/client -h
Usage:
	client [flags] command
Commands:
	- run [run flags] executable [args]
	- output pid
	- stop pid
Flags:
  -addr string
    	Server address in host:port format (default "localhost:8080")
  -ca string
    	CA chain certificate location (default "ca/ca.crt")
  -cert string
    	Client certificate location (default "ca/client_user1.crt")
  -key string
    	Client private key location (default "ca/client_user1.key")
```

Example command execution:  
```
./build/client run bash -c "ps; listmnt /; while true; do echo yes; sleep 1 & wait; done"
PID: 0 
```
`./build/client output 0`

## run command options
Run command help can be found by running  

```
./build/client run -h
Run command flags:
  -cpu uint
    	Set process maximum cpu usage as percentage (default 10)
  -mem uint
    	Set process maximum memory expressed in MB (default 1024)
  -rbps uint
    	Set process maximum read speed in bytes/s (default 10)
  -wbps uint
    	Set process maximum write speed in bytes/s (default 10)
```
