package libsiren

import (
	"bytes"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

var HTTPClient = &http.Client{}

type Options struct {
	Metadata bool
}

func Connect(url string, opt *Options) (*Client, error) {
	if opt == nil {
		opt = &Options{}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// ask for metadata from icecast
	if opt.Metadata {
		req.Header["Icy-Metadata"] = []string{"1"}
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	var metareader *MetadataReader
	var metach chan map[string]string

	if opt.Metadata {
		metareader, err = NewMetadataReader(resp)
		if err != nil {
			return nil, err
		}
		resp.Body = metareader
		metach = metareader.Metadata
	}

	c := Client{
		Response: resp,
		Request:  req,
		Metadata: metach,
	}

	return &c, nil
}

type Client struct {
	Request  *http.Request
	Response *http.Response
	Metadata chan map[string]string
}

func NewMetadataReader(resp *http.Response) (*MetadataReader, error) {
	mr := MetadataReader{
		Body:     resp.Body,
		Metadata: make(chan map[string]string, 1),
	}

	if s := resp.Header.Get("icy-metaint"); s != "" {
		metaint, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		mr.buf = make([]byte, metaint)
		mr.metaint = metaint
	}

	return &mr, nil
}

type MetadataReader struct {
	Body     io.ReadCloser
	Metadata chan map[string]string

	// icecast mp3 metadata interval
	metaint int
	buf     []byte
	databuf bytes.Buffer
}

func (mr *MetadataReader) Read(p []byte) (n int, err error) {
	n, err = mr.databuf.Read(p)
	if err != io.EOF {
		return
	}

	// no data left, so we have to read some and handle metadata
	n, err = io.ReadFull(mr.Body, mr.buf)
	if err != nil {
		return 0, err
	}

	mr.databuf.Write(mr.buf)

	n, err = mr.Body.Read(mr.buf[:1])
	if err != nil {
		return 0, err
	}

	length := int(mr.buf[0]) * 16
	if length == 0 {
		return mr.Read(p)
	}

	n, err = io.ReadFull(mr.Body, mr.buf[:length])
	if err != nil {
		return 0, nil
	}

	handleMetadata(mr.Metadata, mr.buf[:length])
	return mr.Read(p)
}

func (mr *MetadataReader) Close() error {
	return mr.Body.Close()
}

func handleMetadata(ch chan map[string]string, meta []byte) {
	m := map[string]string{}

	re := regexp.MustCompile(`([^=]+)='([^']+?)';`)

	var key, value string
	for _, matches := range re.FindAllSubmatch(meta, -1) {
		key = string(matches[1])
		value = string(matches[2])

		m[key] = value
	}

push:
	for {
		select {
		case ch <- m:
			break push
		case <-ch:
		}
	}
}
