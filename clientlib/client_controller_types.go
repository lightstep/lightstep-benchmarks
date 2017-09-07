package clientlib

import (
	"encoding/json"
	"time"
)

type Duration time.Duration

func (d *Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(*d).String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	s := ""
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if pd, err := time.ParseDuration(s); err != nil {
		return err
	} else {
		*d = Duration(pd)
		return nil
	}
}

func (d Duration) Seconds() float64 {
	return time.Duration(d).Seconds()
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

type (
	TestClientController interface {
		StartControlServer()

		StartClient([]string) error
		//		Run(benchlib.Control) (*benchlib.Result, error)
		StopClient()
	}
)
