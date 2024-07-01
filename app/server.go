package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
)

type HttpRequest struct {
	method string
	target string
	paths []string
	version string
	headers map[string] string
	body string 
}

func ParseHttpRequest(raw string) HttpRequest {
	lines := strings.Split(raw, "\r\n")
	request_line := lines[0]
	request_parts := strings.Split(request_line, " ")

	header_lines := lines[1:len(lines) - 2]
	headers := make(map[string]string, len(header_lines))

	for i := 0; i < len(header_lines); i++ {
		header := header_lines[i]
		split_index := strings.Index(header, ":")
		key, value := strings.TrimSpace(header[0:split_index]), strings.TrimSpace(header[split_index+1:])
		headers[key] = value
	}

	target := request_parts[1]

	return HttpRequest {
		method: request_parts[0],
		target: target,
		paths: strings.Split(target[1:], "/"),
		version: request_parts[2],
		headers: headers,
		body: lines[len(lines)-1],
	}
}

type HttpResponse struct {
	version string
	status_code int
	status_message string
	headers map[string] interface{}
	body []byte
}

func NewHttpResponse() HttpResponse {
	return HttpResponse {
		version: "HTTP/1.1",
		status_code: 200,
		status_message: "OK",
		headers: make(map[string]interface{}),
		body: nil,
	}
}

func (res *HttpResponse) setStatus(code int, message string) *HttpResponse {
	res.status_code = code
	res.status_message = message
	return res
}

func (res *HttpResponse) setBody(body string) *HttpResponse {
	as_bytes := []byte(body)
	res.headers["Content-Type"] = "text/plain"
	res.headers["Content-Length"] = len(as_bytes)
	res.body = as_bytes

	return res
}

func (res *HttpResponse) writable() []byte {
	var buf bytes.Buffer

	// Using sprintf here might negate all benefits of using a string builder but idk tbh
	buf.WriteString(fmt.Sprintf("%s %d %s\r\n", res.version, res.status_code, res.status_message))

	for key, value := range res.headers {
		buf.WriteString(fmt.Sprintf("%s: %v\r\n", key, value))
	}

	buf.WriteString("\r\n")
	buf.Write(res.body)

	return buf.Bytes()
}

func handleConnection(conn net.Conn) {
defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading from connection: ", err.Error())
		os.Exit(1)
	}
	
	request := ParseHttpRequest(string(buf[:n]))
	fmt.Println("Request", request)

	response := NewHttpResponse()

	if len(request.paths) == 2 && request.paths[0] == "echo" {
		response.setBody(request.paths[1])
	} else if request.target == "/user-agent" {
		response.setBody(request.headers["User-Agent"])
	} else if request.target != "/" {
		response.setStatus(404, "Not Found")
	}

	fmt.Println("Response", response)
	conn.Write(response.writable())
}

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	fmt.Println("Listening on port 4221")

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go handleConnection(conn)
	}
}
