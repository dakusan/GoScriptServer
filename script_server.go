// This is a simple web/http [script] server that performs commands via plugins. Commands and their arguments are passed in through URL query parameters.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"script_server/commands"
	_ "script_server/plugins"
	"script_server/settings"
	"script_server/utils"
	"strconv"
	"syscall"
	"time"
)

type errorServerCode int
type errCode struct{ val errorServerCode }

const (
	errorOk  errorServerCode = 0
	errOther                 = iota
	errorHelpString
	errorPort
	errorSettingsFile
	errorOnListen
	errorOnServerClose
	errorServerCloseNoReturn
	errInvalid = -1
)

// Returns a process error code
func main() {
	exitCode := int(runServer().val)
	exitCode = utils.Cond(true, exitCode, errOther) //Used to get rid of warning about errOther not being used
	commands.RunCloseFuncs()
	os.Exit(exitCode)
}

func retInitErr(returnCode errCode, format string, args ...interface{}) errCode {
	fmt.Printf(format+"\n", args...)
	return returnCode
}

func runServer() errCode {
	//Log sends to stdout by default. Errors are directed to stderr
	log.SetOutput(os.Stdout)

	//Make sure arguments exist
	if len(os.Args) < 3 {
		return retInitErr(errCode{errorHelpString}, "Usage: %s PortNumber SecretKey", os.Args[0])
	}

	//Confirm port number
	const minPort, maxPort = 1, 65535
	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return retInitErr(errCode{errorPort}, "Port must be an integer: %s", err.Error())
	} else if port < minPort || port > maxPort {
		return retInitErr(errCode{errorPort}, "Port must be between %d and %d", minPort, maxPort)
	}

	//If settings file does not exit then create it from settings.example.jsonc (if that exists)
	if _, err := os.Stat(settings.FileName); errors.Is(err, os.ErrNotExist) {
		const settingsExampleFileName = "settings.example.jsonc"
		if data, err := os.ReadFile(settingsExampleFileName); err != nil {
			return retInitErr(errCode{errorSettingsFile}, "Could not find %s to convert to %s: %s", settingsExampleFileName, settings.FileName, err)
		} else if err := os.WriteFile(
			settings.FileName,
			utils.IgnoreError(regexp.Compile(`(?m)//.*$`)).ReplaceAll(data, []byte{}),
			0644,
		); err != nil {
			return retInitErr(errCode{errorSettingsFile}, "Could not write settings to %s: %s", settings.FileName, err)
		} else {
			log.Printf("Created %s from %s\n", settings.FileName, settingsExampleFileName)
		}
	}

	if err := settings.InitSettings(); err != nil {
		return retInitErr(errCode{errorSettingsFile}, "Settings file error: %s", err.Error())
	}

	//Create a context that cancels on SIGINT or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	//Create the listener
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return retInitErr(errCode{errorOnListen}, "Failed to listen: %s", err.Error())
	}
	defer func(listener net.Listener) {
		_ = listener.Close()
	}(listener)

	//Create the server
	serverReturnValChan := make(chan errCode)
	server := &http.Server{
		Handler: http.HandlerFunc(handleConnection),
	}
	go func() {
		//If "./cert.pem" and "./key.pem" exist, then use https. Otherwise, use http.
		certFile := settings.Get("Root", "SSLCertificatePath", "./cert.pem")
		keyFile := settings.Get("Root", "SSLKeyPath", "./key.pem")
		runAsHttps := utils.CanAccessFile(certFile) && utils.CanAccessFile(keyFile)
		log.Printf("Starting %s server on port %d", utils.Cond(runAsHttps, "HTTPS", "HTTP"), port)

		//Start the server (is blocking)
		var err error
		if runAsHttps {
			err = server.ServeTLS(listener, certFile, keyFile)
		} else {
			err = server.Serve(listener)
		}

		//After the server ends we get here
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			utils.PrintError("Server error: %s", err.Error())
			serverReturnValChan <- errCode{errorOnServerClose}
		} else {
			log.Println("Server successfully stopped")
			serverReturnValChan <- errCode{errorOk}
		}
	}()

	//Wait for server exit or signal so we can exit cleanly
	shutdownCode := errCode{errInvalid}
	select {
	case val := <-serverReturnValChan:
		log.Println("Shutting down gracefully")
		shutdownCode = val
	case _ = <-ctx.Done():
		log.Println("Received Ctrl+C or SIGTERM, shutting down gracefully")
	}

	//Shut down the server if exiting from a signal
	if shutdownCode.val == errInvalid {
		//Create a context for shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		//Shutdown server, ensuring defers are called
		if err := server.Shutdown(shutdownCtx); err != nil {
			utils.PrintError("Shutdown error: %s", err.Error())
		}

		//Try to get the shutdown code again
		select {
		case val := <-serverReturnValChan:
			shutdownCode = val
		case <-time.After(2 * time.Second):
			shutdownCode = errCode{errorServerCloseNoReturn}
		}
	}

	//Return successful close
	log.Println("Cleanup complete")
	return shutdownCode
}

func handleConnection(w http.ResponseWriter, r *http.Request) {
	//Log the request (remove secret key)
	var requestStr string
	{
		vars := r.URL.Query()
		queryMap := make(url.Values)
		for key, values := range vars {
			queryMap[key] = values
		}
		delete(queryMap, "SecretKey")
		requestStr = queryMap.Encode()
	}

	//Get query value
	vars := r.URL.Query()
	getQueryVal := func(varName string) (string, bool) {
		if val, ok := vars[varName]; !ok {
			return "", false
		} else {
			return val[0], true
		}
	}

	//Output the result and return it to the sender
	startTime := time.Now()
	result := processRequest(getQueryVal)
	utils.CustomLogger(startTime, "%s :: %s", requestStr, result)
	_, _ = w.Write([]byte(result + "\n"))
}

func processRequest(getQueryVal commands.GetQueryValFunc) string {
	//Check for SecretKey and validate
	secretKey := os.Args[2]
	if val, ok := getQueryVal("SecretKey"); !ok || val != secretKey {
		return "Invalid secret key"
	}

	//Handle command key
	if command, ok := getQueryVal("Command"); !ok {
		return "Missing Command"
	} else if cmdFunc, ok := commands.Get(command); !ok {
		return "Invalid Command"
	} else {
		return cmdFunc(getQueryVal)
	}
}
