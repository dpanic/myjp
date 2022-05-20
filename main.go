package main

import (
	"os"

	"github.com/dpanic/myjp/src/api"
)

func main() {
	os.MkdirAll("./logs", 0755)

	api.Run()
}
