# MYJP
[![Go Report Card](https://goreportcard.com/badge/github.com/dpanic/myjp)](https://goreportcard.com/report/github.com/dpanic/myjp)

My Jump Proxy is alternative for socat tcp4 redirect written in Go.

## Features
* Listen on TCP4 IP and PORT, redirect all traffic to remote TCP IP PORT
* Multithreading
* Connection pooling for remote servers

## Configure
Edit configuration on /etc/myjp.conf:
```
0.0.0.0:12345 git-codecommit.ap-southeast-2.amazonaws.com:22
0.0.0.0:54321 google.com:443
```

## Todo
* Validate IP/HOST and PORT for config
* Implement Viper for ENV variables

## Compile
```make build```

## Install
```sudo make install```
