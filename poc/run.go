package poc

import (
	"fmt"
	"golin/global"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var ListPocInfo []Flagcve

type Flagcve struct {
	Url  string
	Cve  string
	Flag string
}

func CheckPoc(url, app string) {
	wg := sync.WaitGroup{}

	if strings.HasSuffix(url, "/") {
		url = url[:len(url)-1]
	}

	app = strings.ToLower(app)

	dirPocs, err := parseConfigs("yaml-poc")
	if err != nil {
		return
	}

	// 这是运行yaml格式的漏洞
	for _, poc := range dirPocs {
		apps := strings.Split(app, ",") // 分割app
		for _, singleApp := range apps {
			if strings.Contains(strings.ToLower(poc.Name), singleApp) && singleApp != "" {
				wg.Add(1)
				go executeRequest(url, poc, &wg)
			}
		}

		if poc.AlwaysExecute {
			wg.Add(1)
			go executeRequest(url, poc, &wg)
		}
	}

	// 这是特定的poc漏洞
	if strings.Contains(app, "spring") {
		CVE_2022_22947(url, "pwd")
	}

	// 这是未授权的漏洞
	authPocs := map[string]Flagcve{
		"elasticsearch[未授权访问]": {url, "elasticsearch未授权访问", "可通过/_cat/indices?v获取所有索引信息"},
		"couchdb":              {url, "CouchDB未授权访问", "可通过/_all_dbs获取所有数据库"},
		"hadoop":               {url, "Hadoop-Administration未授权访问", ""},
		"apache-spark":         {url, "Apache-Spark未授权访问", ""},
		"kafka-manager":        {url, "Kafka-Manager未授权访问", ""},
		"jenkins[未授权访问]":       {url, "jenkins未授权访问", ""},
	}
	for aps, flag := range authPocs {
		if strings.Contains(app, aps) {
			echoFlag(flag)
		}
	}
	wg.Wait()

}

// 基于yaml格式处理http请求
func executeRequest(URL string, config Config, wg *sync.WaitGroup) {
	defer wg.Done()
	for _, path := range config.Path {
		path = replacepath(path) //path中可能有变量进行替换
		baseurl := fmt.Sprintf("%s%s", URL, path)
		values, err := url.ParseQuery(config.Body) //解析body字符串为URL编码
		if err != nil {
			return
		}
		req, err := http.NewRequest(config.Method, baseurl, strings.NewReader(values.Encode()))
		if err != nil {
			return
		}

		for k, v := range config.Headers { //设置header
			req.Header.Set(k, v)
		}

		resp, err := newRequest(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != config.Expression.Status { //状态码判断
			continue
		}

		if config.Expression.ContentType != "" {
			if resp.Header.Get("Content-Type") != config.Expression.ContentType { //返回类型判断
				continue
			}
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		strBody := string(bodyBytes)

		if len(config.Expression.BodyALL) >= 1 {
			if !allSubstringsPresent(strBody, config.Expression.BodyALL) {
				continue
			}
		}

		if len(config.Expression.BodyAny) >= 1 {
			if !anySubstringsPresent(strBody, config.Expression.BodyAny) {
				continue
			}
		}
		if os.Getenv("poc") == "on" {
			fmt.Println(strBody, "\n---------------------")
		}
		flags := Flagcve{baseurl, config.Name, config.Description}
		echoFlag(flags)

	}
}

// replacepath 替换路径中的变量
func replacepath(path string) string {
	nowday := time.Now().Format("06_01_02") //当前日期23_08_22
	path = strings.ReplaceAll(path, "{01_01_01}", nowday)
	return path
}

// allSubstringsPresent 返回值是否同时包含
func allSubstringsPresent(str string, substrings []string) bool {
	for _, substring := range substrings {
		if !strings.Contains(str, substring) {
			return false
		}
	}
	return true
}

// anySubstringsPresent 返回值是否任意包含
func anySubstringsPresent(str string, substrings []string) bool {
	for _, substring := range substrings {
		if strings.Contains(str, substring) {
			return true
		}
	}
	return false
}

func echoFlag(flag Flagcve) {
	global.PrintLock.Lock()
	defer global.PrintLock.Unlock()
	ListPocInfo = append(ListPocInfo, Flagcve{flag.Url, flag.Cve, flag.Flag})
}
