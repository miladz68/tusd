package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sethgrid/pester"
	"github.com/tus/tusd"
)

func checkFileGetAuthHTTP(id string, headers http.Header) bool {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", Flags.HttpHooksEndpoint, id), nil)
	if err != nil {
		return false
	}
	req.Header = headers

	// Use linear backoff strategy with the user defined values.
	client := pester.New()
	client.KeepLog = true
	client.MaxRetries = Flags.HttpHooksRetry
	client.Backoff = func(_ int) time.Duration {
		return time.Duration(Flags.HttpHooksBackoff) * time.Second
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true
	}
	return false

}

func Serve() {
	SetupPreHooks(Composer)

	handler, err := tusd.NewHandler(tusd.Config{
		MaxSize:                 Flags.MaxSize,
		BasePath:                Flags.Basepath,
		RespectForwardedHeaders: Flags.BehindProxy,
		StoreComposer:           Composer,
		NotifyCompleteUploads:   true,
		NotifyTerminatedUploads: true,
		NotifyUploadProgress:    true,
		NotifyCreatedUploads:    true,
		AuthFuncGet:             checkFileGetAuthHTTP,
	})
	if err != nil {
		stderr.Fatalf("Unable to create handler: %s", err)
	}

	address := Flags.HttpHost + ":" + Flags.HttpPort
	basepath := Flags.Basepath

	stdout.Printf("Using %s as address to listen.\n", address)
	stdout.Printf("Using %s as the base path.\n", basepath)

	SetupPostHooks(handler)

	if Flags.ExposeMetrics {
		SetupMetrics(handler)
	}

	stdout.Printf(Composer.Capabilities())

	// Do not display the greeting if the tusd handler will be mounted at the root
	// path. Else this would cause a "multiple registrations for /" panic.
	if basepath != "/" {
		http.HandleFunc("/", DisplayGreeting)
	}

	http.Handle(basepath, http.StripPrefix(basepath, handler))

	timeoutDuration := time.Duration(Flags.Timeout) * time.Millisecond
	listener, err := NewListener(address, timeoutDuration, timeoutDuration)
	if err != nil {
		stderr.Fatalf("Unable to create listener: %s", err)
	}

	if err = http.Serve(listener, nil); err != nil {
		stderr.Fatalf("Unable to serve: %s", err)
	}
}
