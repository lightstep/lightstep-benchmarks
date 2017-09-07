package clientlib

import (
	bench "github.com/lightstep/lightstep-benchmarks/benchlib"

	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"
)

var allClients = map[string][]string{
	"cpp":    []string{"./cppclient"},
	"ruby":   []string{"ruby", "./benchmark.rb"},
	"python": []string{"./pyclient.py"},
	"golang": []string{"./goclient"},
	"nodejs": []string{"node",
		"--expose-gc",
		"--always_opt",
		//"--trace-gc", "--trace-gc-verbose", "--trace-gc-ignore-scavenger",
		"./jsclient.js"},
	"java": []string{
		"java",
		// "-classpath",
		// "lightstep-benchmark-0.1.28.jar",
		//"-Xdebug", "-Xrunjdwp:transport=dt_socket,address=7000,server=y,suspend=n",

		// works with VisualVM... replace your localhost IP address as the RMI SERVER HOSTNAME
		// aka the thing you get from ifconfig
		//"-Dcom.sun.management.jmxremote",
		//"-Dcom.sun.management.jmxremote.port=9010",
		//"-Dcom.sun.management.jmxremote.rmi.port=9110",
		//"-Dcom.sun.management.jmxremote.local.only=false",
		//"-Dcom.sun.management.jmxremote.authenticate=false",
		//"-Dcom.sun.management.jmxremote.ssl=false",
		//"-Djava.rmi.server.hostname=192.168.27.38",

		"com.lightstep.benchmark.BenchmarkClient"},
}

type sreq struct {
	w      http.ResponseWriter
	r      *http.Request
	doneCh chan struct{}
}

var requestCh = make(chan sreq)

func serializeHTTP(w http.ResponseWriter, r *http.Request) {
	doneCh := make(chan struct{})
	requestCh <- sreq{w, r, doneCh}
	<-doneCh
}

func CreateHTTPTestClientController() TestClientController {
	return &HTTPTestClientController{}
}

type HTTPTestClientController struct {
	controlCh chan *bench.Control
	resultCh  chan *bench.Result

	clientStopped chan bool

	// Params
	TestTimeSlice             Duration
	UserInterferenceThreshold float64
	SysInterferenceThreshold  float64

	// Client
	clientProcess *exec.Cmd
	controlling   bool

	// Client stats
	before     bench.Timing
	beforeSelf bench.Timing
	beforeStat bench.CPUStat
}

func (c *HTTPTestClientController) StartClient(command []string) error {
	c.clientStopped = make(chan bool)
	c.clientProcess = exec.Command(command[0], command[1:]...)
	c.clientProcess.Stderr = os.Stderr
	c.clientProcess.Stdout = os.Stdout
	if err := c.clientProcess.Start(); err != nil {
		bench.Fatal("Could not start client: ", err)
	}
	// Start watch goroutine
	go func() {
		if err := c.clientProcess.Wait(); err != nil {
			perr, ok := err.(*exec.ExitError)
			if !ok {
				bench.Fatal("Could not await client: ", err)
			}
			if !perr.Exited() {
				bench.Fatal("Client did not exit: ", err)
			}
			if !perr.Success() {
				bench.Fatal("Client failed: ", string(perr.Stderr))
			}
		}
		c.clientStopped <- true
	}()

	return nil
}

func (c *HTTPTestClientController) StopClient() {
	c.controlCh <- &bench.Control{Exit: true}
	<-c.clientStopped
	c.controlling = false
}

func (c *HTTPTestClientController) StartControlServer() {
	address := fmt.Sprintf(":%v", bench.ControllerPort)
	mux := http.NewServeMux()
	// Note: the 100000 second timeout avoids HTTP disconnections,
	// which can confuse very simple HTTP libraries (e.g., the C++
	// benchmark client).
	server := &http.Server{
		Addr:         address,
		ReadTimeout:  100000 * time.Second,
		WriteTimeout: 0 * time.Second,
		Handler:      http.HandlerFunc(serializeHTTP),
	}

	c.resultCh = make(chan *bench.Result)
	c.controlCh = make(chan *bench.Control)

	if c.UserInterferenceThreshold == 0 {
		c.UserInterferenceThreshold = 0.01
	}
	if c.SysInterferenceThreshold == 0 {
		c.SysInterferenceThreshold = 0.02
	}
	mux.HandleFunc(bench.ControlPath, c.serveControlHTTP)
	mux.HandleFunc(bench.ResultPath, c.serveResultHTTP)
	mux.HandleFunc("/", c.serveDefaultHTTP)

	go func() {
		for req := range requestCh {
			mux.ServeHTTP(req.w, req.r)
			close(req.doneCh)
		}
	}()

	go func() {
		bench.Fatal(server.ListenAndServe())
	}()
}

func (c *HTTPTestClientController) serveDefaultHTTP(res http.ResponseWriter, req *http.Request) {
	bench.Fatal("Unexpected HTTP request", req.URL)
}

func (c *HTTPTestClientController) serveControlHTTP(res http.ResponseWriter, req *http.Request) {

	if c.controlling {
		bench.Fatal("Out-of-phase control request", req.URL)
	}

	c.before, c.beforeSelf, c.beforeStat = bench.GetChildUsage(c.clientProcess.Process.Pid)
	c.controlling = true
	control := <-c.controlCh
	body, err := json.Marshal(control)
	if err != nil {
		bench.Fatal("Marshal error: ", err)
	}
	_, err = res.Write(body)
	if err != nil {
		bench.Fatal("Response write error: ", err)
	}
}

func (c *HTTPTestClientController) serveResultHTTP(res http.ResponseWriter, req *http.Request) {

	if !c.controlling {
		bench.Fatal("Out-of-phase client result", req.URL)
	}

	benchResult := c.formResult(req)
	c.controlling = false
	c.resultCh <- benchResult

	// The response body is not used, but some HTTP clients are
	// troubled by 0-byte responses.
	_, err := res.Write([]byte("OK"))
	if err != nil {
		bench.Fatal("Response write error: ", err)
	}

}

func (c *HTTPTestClientController) formResult(req *http.Request) *bench.Result {
	usage, usageSelf, usageStat := bench.GetChildUsage(c.clientProcess.Process.Pid)
	usage = usage.Sub(c.before)
	usageSelf = usageSelf.Sub(c.beforeSelf)

	// Note: it would be nice if there were a decoder to unmarshal
	// from URL query param into bench.Result, e.g., opposite of
	// https://godoc.org/github.com/google/go-querystring/query
	params, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		bench.Fatal("Error parsing URL params: ", req.URL.RawQuery)
	}

	// Look for CPU contention on the machine. (TODO 100 == Hz)
	if usage.User.Seconds() > c.TestTimeSlice.Seconds() {
		osUser := bench.Time(float64(usageStat.User-c.beforeStat.User) / 100)
		osSys := bench.Time(float64(usageStat.System-c.beforeStat.System) / 100)

		stolenTicks := usageStat.Steal - c.beforeStat.Steal
		if stolenTicks != 0 {
			bench.Print("Stolen ticks! It's unfair!", stolenTicks)
			return nil
		}

		du := osUser - usage.User - usageSelf.User
		if (du / osUser).Seconds() > c.UserInterferenceThreshold {
			bench.Print(fmt.Sprintf("User interference: %0.1f%% [%.3f/%.3f]", 100*float64(du/osUser), du, usage.User))
			return nil
		}
		ds := osSys - usage.Sys - usageSelf.Sys
		// Compare other system activity against the process's user time
		if (ds / usage.User).Seconds() > c.SysInterferenceThreshold {
			bench.Print(fmt.Sprintf("System interference: %0.1f%% [%.3f/%.3f]", 100*float64(ds/usage.User), ds, usage.User))
			return nil
		}
	}

	return &bench.Result{
		Measured: bench.Timing{
			Wall: bench.ParseTime(params.Get("timing")),
			User: usage.User,
			Sys:  usage.Sys,
		},
		Flush: bench.Timing{
			Wall: bench.ParseTime(params.Get("flush")),
		},
		Sleeps: bench.ParseTime(params.Get("s")),
	}
}
