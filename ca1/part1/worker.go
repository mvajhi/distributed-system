package main

import (
	"bufio"
	"encoding/json"
	"log"
	"math"
	"os"
	"strings"
	"syscall"
)

type Request struct {
	Op string  `json:"op"`
	A  float64 `json:"a"`
	B  float64 `json:"b"`
}

type Response struct {
	Status  string  `json:"status"`
	Result  float64 `json:"result,omitempty"`
	Message string  `json:"message,omitempty"`
}

const reqPipeFile = "/tmp/hw1_req_pipe"
const resPipeFile = "/tmp/hw1_res_pipe"

func setupPipe(path string) {
	os.Remove(path)
	err := syscall.Mkfifo(path, 0666)
	if err != nil {
		log.Fatalf("Failed to create pipe %s: %v\n", path, err)
	}
}

func main() {
	setupPipe(reqPipeFile)
	setupPipe(resPipeFile)
	defer os.Remove(reqPipeFile)
	defer os.Remove(resPipeFile)

	log.Println("Worker process started. Waiting for interface connection...")

	reqPipe, err := os.OpenFile(reqPipeFile, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Fatalf("Error opening request pipe: %v\n", err)
	}
	defer reqPipe.Close()

	resPipe, err := os.OpenFile(resPipeFile, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		log.Fatalf("Error opening response pipe: %v\n", err)
	}
	defer resPipe.Close()

	log.Println("Connected. Ready to process requests...")

	reader := bufio.NewReader(reqPipe)
	encoder := json.NewEncoder(resPipe)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Pipe disconnected. Shutting down worker...")
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req Request
		var res Response

		err = json.Unmarshal([]byte(line), &req)
		if err != nil {
			res = Response{Status: "ERR", Message: "invalid_json"}
			encoder.Encode(res)
			continue
		}

		switch strings.ToUpper(req.Op) {
		case "ADD":
			res = Response{Status: "OK", Result: req.A + req.B}
		case "SUB":
			res = Response{Status: "OK", Result: req.A - req.B}
		case "MUL":
			res = Response{Status: "OK", Result: req.A * req.B}
		case "DIV":
			if req.B == 0 {
				res = Response{Status: "ERR", Message: "division_by_zero"}
			} else {
				res = Response{Status: "OK", Result: req.A / req.B}
			}
		case "MOD":
			if req.B == 0 {
				res = Response{Status: "ERR", Message: "division_by_zero"}
			} else {
				res = Response{Status: "OK", Result: math.Mod(req.A, req.B)}
			}
		case "POW":
			res = Response{Status: "OK", Result: math.Pow(req.A, req.B)}
		default:
			res = Response{Status: "ERR", Message: "unknown_command"}
		}

		err = encoder.Encode(res)
		if err != nil {
			log.Printf("Error writing response: %v\n", err)
		}
	}
}