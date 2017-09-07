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

	err := controller.StartClient([]string{"./goclient"})
	if err != nil {
		t.Error("Should not have errored")
	}
	controller.StopClient()
}
