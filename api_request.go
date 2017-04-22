package nsq

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// 定义带有超时的连接对象
type deadlinedConn struct {
	Timeout time.Duration
	net.Conn
}

// 实现io.Reader接口,带有超时
func (c *deadlinedConn) Read(b []byte) (n int, err error) {
	c.Conn.SetReadDeadline(time.Now().Add(c.Timeout))
	return c.Conn.Read(b)
}
// 实现io.Writer接口,带有超时
func (c *deadlinedConn) Write(b []byte) (n int, err error) {
	c.Conn.SetWriteDeadline(time.Now().Add(c.Timeout))
	return c.Conn.Write(b)
}

// 创建带有超时的http.Transport对象，实现是因为Dial是deadlinedConn类型
func newDeadlineTransport(timeout time.Duration) *http.Transport {
	transport := &http.Transport{
		Dial: func(netw, addr string) (net.Conn, error) {
			c, err := net.DialTimeout(netw, addr, timeout)
			if err != nil {
				return nil, err
			}
			return &deadlinedConn{timeout, c}, nil
		},
	}
	return transport
}

// 定义回复结构
type wrappedResp struct {
	Status     string      `json:"status_txt"`
	StatusCode int         `json:"status_code"`
	Data       interface{} `json:"data"`
}

// stores the result in the value pointed to by ret(must be a pointer)
// 发起请求，并将结果保持到ret结构(must be a pointer)
func apiRequestNegotiateV1(method string, endpoint string, body io.Reader, ret interface{}) error {
	// 创建Http Client对象
	httpclient := &http.Client{Transport: newDeadlineTransport(2 * time.Second)}
	// 创建Http 请求对象
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", "application/vnd.nsq; version=1.0")
	// 发送Http请求，并获取结果(带有超时)
	resp, err := httpclient.Do(req)
	if err != nil {
		return err
	}
	// 读取全部的Body信息
	respBody, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	// 如果返回不是200就认为出错
	if resp.StatusCode != 200 {
		return fmt.Errorf("got response %s %q", resp.Status, respBody)
	}

	if len(respBody) == 0 {
		respBody = []byte("{}")
	}

	if resp.Header.Get("X-NSQ-Content-Type") == "nsq; version=1.0" {
		return json.Unmarshal(respBody, ret)
	}

	wResp := &wrappedResp{
		Data: ret,
	}

	if err = json.Unmarshal(respBody, wResp); err != nil {
		return err
	}

	// wResp.StatusCode here is equal to resp.StatusCode, so ignore it
	return nil
}
