package clientlib

import (
	"fmt"
	"testing"
)

func TestClientStart(t *testing.T) {
	controller := CreateHTTPTestClientController()
	fmt.Printf("Starting Control server")
	controller.StartControlServer()
	fmt.Printf("Control server started")

	client := CreateProcessClient([]string{"./goclient"})
	err := controller.StartClient(client)
	if err != nil {
		t.Error("Should not have errored")
	}
	controller.StopClient()
}
