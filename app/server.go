package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net"
	"os"
	"strings"
)

type HttpRequestContext struct {
	conn net.Conn
	req HttpRequest
	res HttpResponse
	dir string
}

func (ctx *HttpRequestContext) sendResponse() {
	_, err := ctx.conn.Write(ctx.res.writable())
	if err != nil {
		fmt.Println("Error sending http response: ", err.Error())
	}
	ctx.conn.Close()
}

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

func (res *HttpResponse) setBodyFile(abspath string) *HttpResponse {
	file, err := os.ReadFile(abspath)
	if err != nil {
		res.setStatus(404, "Not Found")
		res.setBody(fmt.Sprintf("Error reading file: %s", err.Error()))
		return res;
	}

	res.headers["Content-Type"] = "application/octet-stream"
	res.headers["Content-Length"] = len(file)
	res.body = file

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

func handleRequest(ctx HttpRequestContext) {
	defer ctx.conn.Close()

	accept_encoding, ok := ctx.req.headers["Accept-Encoding"]
	if ok {
		schemes := strings.Split(accept_encoding, ",")
		for i := 0; i < len(schemes); i++ {
			scheme := strings.TrimSpace(schemes[i])
			if scheme == "gzip" {
				ctx.res.headers["Content-Encoding"] = scheme
				break
			}
		}
	}

	if ctx.req.method == "GET" {
		if len(ctx.req.paths) == 2 {
			if ctx.req.paths[0] == "echo" {
				ctx.res.setBody(ctx.req.paths[1])
			} else if ctx.req.paths[0] == "files" && ctx.dir != "" {
				ctx.res.setBodyFile(fmt.Sprintf("%s%s", ctx.dir, ctx.req.paths[1]))
			}
		} else if ctx.req.target == "/user-agent" {
			ctx.res.setBody(ctx.req.headers["User-Agent"])
		} else if ctx.req.target != "/" {
			ctx.res.setStatus(404, "Not Found")
		}
	} else if ctx.req.method == "POST" {
		if len(ctx.req.paths) == 2 && ctx.req.paths[0] == "files" {
			abspath := fmt.Sprintf("%s%s", ctx.dir, ctx.req.paths[1])
			err := os.WriteFile(abspath, []byte(ctx.req.body), 0644)
			if err != nil {
				ctx.res.setStatus(400, "Bad Request")
				ctx.res.setBody(fmt.Sprintf("Error writing file: %s", err.Error()))
			} else {
				ctx.res.setStatus(201, "Created")
			}
		}
	}

	target_encoding, ok := ctx.res.headers["Content-Encoding"]
	if ok && target_encoding == "gzip" && ctx.res.body != nil {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)

		_, err := zw.Write(ctx.res.body)
		if err != nil {
			fmt.Println("Error compressing body: ", err.Error())
		}
		zw.Close()

		compressed_body := buf.Bytes()
		ctx.res.headers["Content-Length"] = len(compressed_body)
		ctx.res.body = compressed_body
	}

	ctx.sendResponse()
}

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Program arguments", os.Args)
	fmt.Println("Logs from your program will appear here!")

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	fmt.Println("Listening on port 4221")

	var dir = ""
	for i := 0; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--directory" {
			dir = os.Args[i + 1]
			i++
		}
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading from connection: ", err.Error())
			os.Exit(1)
		}

		ctx := HttpRequestContext {
			conn: conn,
			req: ParseHttpRequest(string(buf[:n])),
			res: NewHttpResponse(),
			dir: dir,
		}


		go handleRequest(ctx)
	}
}
