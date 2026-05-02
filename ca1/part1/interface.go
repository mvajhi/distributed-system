package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
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

func main() {
	if _, err := os.Stat(reqPipeFile); os.IsNotExist(err) {
		log.Fatalf("Worker is not running! Please start worker.go first.\n")
	}

	reqPipe, err := os.OpenFile(reqPipeFile, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		log.Fatalf("Error opening request pipe: %v\n", err)
	}
	defer reqPipe.Close()

	resPipe, err := os.OpenFile(resPipeFile, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Fatalf("Error opening response pipe: %v\n", err)
	}
	defer resPipe.Close()

	fmt.Println("Interface ready. Enter operations (e.g., ADD 5 7) or EXIT:")

	stdinReader := bufio.NewReader(os.Stdin)
	pipeReader := bufio.NewReader(resPipe)

	for {
		fmt.Print("> ")
		input, err := stdinReader.ReadString('\n')
		if err != nil {
			log.Println("Input error:", err)
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if strings.ToUpper(input) == "EXIT" {
			break
		}

		parts := strings.Fields(input)
		if len(parts) != 3 {
			fmt.Println("ERR invalid_argument_count")
			continue
		}

		op := parts[0]
		a, errA := strconv.ParseFloat(parts[1], 64)
		b, errB := strconv.ParseFloat(parts[2], 64)

		if errA != nil || errB != nil {
			fmt.Println("ERR non_numeric_input")
			continue
		}

		req := Request{Op: op, A: a, B: b}
		reqBytes, _ := json.Marshal(req)

		_, err = reqPipe.WriteString(string(reqBytes) + "\n")
		if err != nil {
			log.Println("ERR pipe_broken")
			break
		}

		resLine, err := pipeReader.ReadString('\n')
		if err != nil {
			log.Println("ERR unexpected_closure")
			break
		}

		var res Response
		err = json.Unmarshal([]byte(resLine), &res)
		if err != nil {
			fmt.Println("ERR invalid_response_format")
			continue
		}

		if res.Status == "OK" {
			fmt.Printf("OK %v\n", res.Result)
		} else {
			fmt.Printf("ERR %s\n", res.Message)
		}
	}
}