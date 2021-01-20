package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/lucas-clemente/quic-go/http3"
)

// http/3 client
var h3client = &http.Client{
	Transport: &http3.RoundTripper{},
}

// http/2 client
var h2client = &http.Client{}

// user agent to use
var ua = "Mozilla/5.0 (Windows NT 10.0; rv:78.0) Gecko/20100101"

func genericHTTPProxy(w http.ResponseWriter, req *http.Request) {

	q := req.URL.Query()
	host := q.Get("host")
	q.Del("host")

	if len(host) <= 0 {
		host = q.Get("hls_chunk_host")
	}

	if len(host) <= 0 {
		host = getHost(req.URL.Path)
	}

	if len(host) <= 0 {
		io.WriteString(w, "No host in query parameters.")
		return
	}

	path := req.URL.Path

	path = strings.Replace(path, "/ggpht", "", 1)
	path = strings.Replace(path, "/i/", "/", 1)

	proxyURL, err := url.Parse("https://" + host + path)

	if err != nil {
		log.Panic(err)
	}

	proxyURL.RawQuery = q.Encode()

	if strings.HasSuffix(proxyURL.Path, "maxres.jpg") {
		proxyURL.Path = getBestThumbnail(proxyURL.Path)
	}

	request, err := http.NewRequest("GET", proxyURL.String(), nil)

	copyHeaders(req.Header, request.Header)
	request.Header.Set("User-Agent", ua)

	if err != nil {
		log.Panic(err)
	}

	var client *http.Client

	// https://github.com/lucas-clemente/quic-go/issues/2836
	client = h2client

	resp, err := client.Do(request)

	if err != nil {
		log.Panic(err)
	}

	copyHeaders(resp.Header, w.Header())

	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
	resp.Body.Close()
}

func copyHeaders(from http.Header, to http.Header) {
	// Loop over header names
	for name, values := range from {
		// Loop over all values for the name.
		for _, value := range values {
			to.Set(name, value)
		}
	}
}

func getHost(path string) (host string) {

	host = ""

	if strings.HasPrefix(path, "/vi/") || strings.HasPrefix(path, "/vi_webp/") || strings.HasPrefix(path, "/sb/") {
		host = "i.ytimg.com"
	}

	if strings.HasPrefix(path, "/ggpht/") {
		host = "yt3.ggpht.com"
	}

	if strings.HasPrefix(path, "/a/") || strings.HasPrefix(path, "/ytc/") {
		host = "yt3.ggpht.com"
	}

	return host
}

func getBestThumbnail(path string) (newpath string) {

	formats := [4]string{"maxresdefault.jpg", "sddefault.jpg", "hqdefault.jpg", "mqdefault.jpg"}

	for _, format := range formats {
		newpath = strings.Replace(path, "maxres.jpg", format, 1)
		url := "https://i.ytimg.com" + newpath
		resp, _ := h2client.Head(url)
		if resp.StatusCode == 200 {
			return newpath
		}
	}

	return strings.Replace(path, "maxres.jpg", "mqdefault.jpg", 1)
}

func main() {
	http.HandleFunc("/", genericHTTPProxy)
	socket := "socket" + string(os.PathSeparator) + "http-proxy.sock"
	syscall.Unlink(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		fmt.Println("Failed to bind to UDS, falling back to TCP/IP")
		fmt.Println(err.Error())
		http.ListenAndServe(":8080", nil)
	} else {
		http.Serve(listener, nil)
	}
}
