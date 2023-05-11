package main

import (
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	runtime "github.com/banzaicloud/logrus-runtime-formatter"
	log "github.com/sirupsen/logrus"
)

var (
	flagDataPath string
	flagCertFile string
	flagKeyFile  string

	flagListenAddress string
)

func init() {
	flag.StringVar(&flagCertFile, "certfile", "", "path to the certs file for https")
	flag.StringVar(&flagKeyFile, "keyfile", "", "path to the keyfile for https")
	flag.StringVar(&flagDataPath, "data_path", os.TempDir(), "path to the data storage directory")

	flag.StringVar(&flagListenAddress, "listen_address", "0.0.0.0:8080", "address:port to bind listener on")

	// Log as JSON instead of the default ASCII formatter, but wrapped with the runtime Formatter.
	formatter := runtime.Formatter{ChildFormatter: &log.JSONFormatter{}}
	// Enable line number logging as well
	formatter.Line = true

	// Replace the default Logrus Formatter with our runtime Formatter
	//log.SetFormatter(&formatter)
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the info severity or above.
	log.SetLevel(log.InfoLevel)
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

    IPAddress := r.Header.Get("X-Real-Ip")
    if IPAddress == "" {
    	IPAddress = r.Header.Get("X-Forwarded-For")
    }
    if IPAddress == "" {
    	IPAddress = r.RemoteAddr
    }
    var rip string
    rip = IPAddress
    ip := strings.Split(rip, ":")
    
    IPAddress = ip[0]
    
    // initial logs
    log.WithFields(log.Fields{
    	"remote_addr": IPAddress,
    	"method":      r.Method,
    	"request_uri": r.RequestURI,
		"user_agent": r.Header.Get("User-Agent"),
    }).Info("")
    
    handler.ServeHTTP(w, r)
    
    })
}

func requestHandler(res http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/" {
		http.NotFound(res, req)
		return
	}

	stateStorageFile := filepath.Join(flagDataPath, req.URL.Path)
	stateStorageDir := filepath.Dir(stateStorageFile)


	switch req.Method {
	case "GET":
		fh, err := os.Open(stateStorageFile)
		if err != nil {
			log.Printf("cannot open file: %s\n", err)
			goto not_found
		}
		defer fh.Close()

		res.WriteHeader(200)

		io.Copy(res, fh)

		return
	case "POST":
		if err := os.MkdirAll(stateStorageDir, 0750); err != nil && !os.IsExist(err) {
			log.Printf("cannot create parent directories: %s\n", err)
			goto not_found
		}

		fh, err := os.OpenFile(stateStorageFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			log.Printf("cannot open file: %s\n", err)
			goto not_found
		}
		defer fh.Close()

		if _, err := io.Copy(fh, req.Body); err != nil {
			log.Printf("failed to stream data into statefile: %s\n", err)
			goto not_found
		}

		res.WriteHeader(200)

		return
	case "DELETE":
		if  err := os.RemoveAll(stateStorageFile); err != nil {
			log.Printf("cannot remove file: %s\n", err)
			goto not_found
		}

		res.WriteHeader(200)

		return
	}

not_found:
	http.NotFound(res, req)
}

func main() {
	if !flag.Parsed() {
		flag.Parse()
	}

	http.HandleFunc("/", requestHandler)

	if flagCertFile != "" && flagKeyFile != "" {
		http.ListenAndServeTLS(flagListenAddress, flagCertFile, flagKeyFile,  logRequest(http.DefaultServeMux))
	} else {
		http.ListenAndServe(flagListenAddress, logRequest(http.DefaultServeMux))
	}
}

