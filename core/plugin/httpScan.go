package plugin

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"separa/pkg"

	"github.com/chainreactors/logs"
)

var headers = http.Header{
	"User-Agent": []string{"Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; Trident/5.0;"},
}

// -default
// socket进行对网站的连接
func initScan(result *pkg.Result) {
	var bs []byte
	target := result.GetTarget()
	if pkg.ProxyUrl != nil && strings.HasPrefix(pkg.ProxyUrl.Scheme, "http") {
		// 如果是http代理, 则使用http库代替socket
		conn := result.GetHttpConn(2)
		req, _ := http.NewRequest("GET", "http://"+target, nil)
		resp, err := conn.Do(req)
		if err != nil {
			result.Error = err.Error()
			return
		}
		result.Open = true
		pkg.CollectHttpInfo(result, resp)
	} else {
		conn, err := pkg.NewSocket("tcp", target, 2)
		//conn, err := pkg.TcpSocketConn(target, 2)
		if err != nil {
			// return open: 0, closed: 1, filtered: 2, noroute: 3, denied: 4, down: 5, error_host: 6, unkown: -1
			errMsg := err.Error()
			result.Error = errMsg
			if RunOpt.Debug {
				if strings.Contains(errMsg, "refused") {
					result.ErrStat = 1
				} else if strings.Contains(errMsg, "timeout") {
					result.ErrStat = 2
				} else if strings.Contains(errMsg, "no route to host") {
					result.ErrStat = 3
				} else if strings.Contains(errMsg, "permission denied") {
					result.ErrStat = 4
				} else if strings.Contains(errMsg, "host is down") {
					result.ErrStat = 5
				} else if strings.Contains(errMsg, "no such host") {
					result.ErrStat = 6
				} else if strings.Contains(errMsg, "network is unreachable") {
					result.ErrStat = 6
				} else if strings.Contains(errMsg, "The requested address is not valid in its context.") {
					result.ErrStat = 6
				} else {
					result.ErrStat = -1
				}
			}
			return
		}
		defer conn.Close()
		result.Open = true

		// 启发式扫描探测直接返回不需要后续处理
		if result.SmartProbe {
			return
		}
		result.Status = "tcp"

		bs, err = conn.Read(1)
		if err != nil {
			senddataStr := fmt.Sprintf("GET /%s HTTP/1.1\r\nHost: %s\r\n\r\n", result.Uri, target)
			bs, err = conn.Request([]byte(senddataStr), 4096)
			if err != nil {
				result.Error = err.Error()
			}
		}
		pkg.CollectSocketInfo(result, bs)
	}

	//所有30x,400,以及非http协议的开放端口都送到http包尝试获取更多信息
	if result.Status == "400" || result.Protocol == "tcp" || (strings.HasPrefix(result.Status, "3") && bytes.Contains(result.Content, []byte("location: https"))) {
		systemHttp(result, "https")
	} else if strings.HasPrefix(result.Status, "3") {
		systemHttp(result, "http")
	}
	return
}

// 使用net/http进行带redirect的请求
func systemHttp(result *pkg.Result, scheme string) {

	// 如果是400或者不可识别协议,则使用https
	target := scheme + "://" + result.GetTarget()

	//if RunOpt.SuffixStr != "" {
	//	target += "/" + RunOpt.SuffixStr
	//}

	conn := result.GetHttpConn(2 + 2)
	req, _ := http.NewRequest("GET", target, nil)
	req.Header = headers

	resp, err := conn.Do(req)
	if err != nil {
		// 有可能存在漏网之鱼, 是tls服务, 但tls的第一个响应为30x, 并30x的目的地址不可达或超时. 则会报错.
		result.Error = err.Error()
		logs.Log.Debugf("request %s , %s ", target, err.Error())
		noRedirectHttp(result, req)
		return
	}
	logs.Log.Debugf("request %s , %d ", target, resp.StatusCode)
	if resp.TLS != nil {
		if result.Status == "400" || (resp.Request.Response != nil && resp.Request.Response.StatusCode != 302) || resp.Request.Response == nil {
			// 1. 如果第一个包的状态码为400, 且这个包存在tls, 则判断为https
			// 2. 去掉302跳转到https导致可能存在的误判
			result.Protocol = "https"
		}

		collectTLS(result, resp)
	} else if resp.Request.Response != nil && resp.Request.Response.TLS != nil {
		// 一种相对罕见的情况, 从https页面30x跳转到http页面. 则判断tls
		result.Protocol = "https"

		collectTLS(result, resp.Request.Response)
	} else {
		result.Protocol = "http"
	}

	result.Error = ""
	pkg.CollectHttpInfo(result, resp)
	return
}

// 302跳转后目的不可达时进行不redirect的信息收集
// 暂时使用不太优雅的方案, 在极少数情况下才会触发, 会多进行一次https的交互.
func noRedirectHttp(result *pkg.Result, req *http.Request) {
	conn := pkg.HttpConnWithNoRedirect(2 + 2)
	req.Header = headers
	resp, err := conn.Do(req)
	if err != nil {
		// 有可能存在漏网之鱼, 是tls服务, 但tls的第一个响应为30x, 并30x的目的地址不可达或超时. 则会报错.
		result.Error = err.Error()
		logs.Log.Debugf("request (no redirect) %s , %s ", req.URL.String(), err.Error())
		return
	}

	logs.Log.Debugf("request (no redirect) %s , %d ", req.URL.String(), resp.StatusCode)
	if resp.TLS != nil {
		if result.Status == "400" {
			result.Protocol = "https"
		}

		collectTLS(result, resp)
	} else {
		result.Protocol = "http"
	}

	result.Error = ""
	pkg.CollectHttpInfo(result, resp)
}

func collectTLS(result *pkg.Result, resp *http.Response) {
	result.Host = strings.Join(resp.TLS.PeerCertificates[0].DNSNames, ",")
	if len(resp.TLS.PeerCertificates[0].DNSNames) > 0 {
		// 经验公式: 通常只有cdn会绑定超过2个host, 正常情况只有一个host或者带上www的两个host
		result.HttpHosts = append(result.HttpHosts, pkg.FormatCertDomains(resp.TLS.PeerCertificates[0].DNSNames)...)
	}
}