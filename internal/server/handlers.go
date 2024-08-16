package server

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type ocrRequest struct {
	ImagePath     string `json:"image_path,omitempty"`
	Base64Content string `json:"image_base64,omitempty"`
}

type ocrResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func (s *Server) handleOCR(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/stats" {
		log.Println("Received request for server stats")
		stats := s.GetStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
		return
	}

	if r.Method != http.MethodPost {
		log.Printf("Received unsupported method: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ocrRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		http.Error(w, "Error parsing JSON", http.StatusBadRequest)
		return
	}

	if req.ImagePath == "" && req.Base64Content == "" {
		log.Println("Received request with missing image data")
		http.Error(w, "Missing image_path or image_base64 parameter", http.StatusBadRequest)
		return
	}

	log.Println("Received OCR request, queueing task")
	task := ocrTask{
		ImagePath: req.ImagePath,
		Response:  make(chan ocrResponse, 1),
	}

	if req.Base64Content != "" {
		imageData, err := base64.StdEncoding.DecodeString(req.Base64Content)
		if err != nil {
			log.Printf("Invalid base64 image data: %v", err)
			http.Error(w, "Invalid base64 image data", http.StatusBadRequest)
			return
		}
		task.ImageData = imageData
	}

	select {
	case s.taskQueue <- task:
		log.Println("Task queued successfully")
		response := <-task.Response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	case <-time.After(10 * time.Second):
		log.Println("Task queue is full, request timed out")
		http.Error(w, "Server is too busy, please try again later", http.StatusServiceUnavailable)
	}
}