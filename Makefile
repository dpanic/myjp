all: clean build 

clean:
	mkdir -p bin
	rm -rf bin/myjp

build:
	GODEBUG="madvdontneed=1" CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -o bin/myjp


install: 
	cp -r conf/* /etc/
	cp -r bin/myjp /usr/local/bin
	chmod +x /usr/local/bin