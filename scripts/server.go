package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

type CopyFileRequest struct {
	ContainerID       string `json:"container_id"`
	ContainerFilePath string `json:"container_file_path"`
	HostFilePath      string `json:"host_file_path"`
}

func main() {
	// Define a flag for the port
	port := flag.String("port", "8080", "port to listen on")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/copyfile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var req CopyFileRequest
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&req)
		if err != nil {
			http.Error(w, "Failed to parse request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Check if all required parameters are provided
		if req.ContainerID == "" || req.ContainerFilePath == "" || req.HostFilePath == "" {
			http.Error(w, "Missing required parameter(s)", http.StatusBadRequest)
			return
		}

		// Command to copy the file from the container to the host
		cmd := exec.Command("docker", "cp", req.ContainerID+":"+req.ContainerFilePath, req.HostFilePath)
		err = cmd.Run()
		if err != nil {
			io.WriteString(w, "Failed to copy file: "+err.Error())
			http.Error(w, "Failed to copy file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Respond with a success message
		io.WriteString(w, "File copied successfully!")
	})

	mux.HandleFunc("/copydir", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var req CopyFileRequest
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&req)
		if err != nil {
			http.Error(w, "Failed to parse request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Check if all required parameters are provided
		if req.ContainerID == "" || req.ContainerFilePath == "" || req.HostFilePath == "" {
			http.Error(w, "Missing required parameter(s)", http.StatusBadRequest)
			return
		}

		// Command to copy the directory from the container to the host
		cmd := exec.Command("docker", "cp", req.ContainerID+":"+req.ContainerFilePath, req.HostFilePath)
		err = cmd.Run()
		if err != nil {
			io.WriteString(w, "Failed to copy directory: "+err.Error())
			http.Error(w, "Failed to copy directory: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Respond with a success message
		io.WriteString(w, "Directory copied successfully!")
	})

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)

	server := &http.Server{Addr: ":" + *port, Handler: mux}

	go func() {
		log.Printf("Starting server on :%s\n", *port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on :%s: %v\n", *port, err)
		}
	}()

	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		io.WriteString(w, "Server is stopping...")

		// Signal the server to shutdown
		stopChan <- os.Interrupt
	})

	<-stopChan
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
