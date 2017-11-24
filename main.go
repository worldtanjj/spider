package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"sync"
	"time"
)

var (
	host       = "http://www.sihu888.com/"
	urlChannel = make(chan string, 1)
	r          = rand.New(rand.NewSource(time.Now().UnixNano()))
	userAgent  = []string{"Mozilla/5.0 (compatible, MSIE 10.0, Windows NT, DigExt)",
		"Mozilla/4.0 (compatible, MSIE 7.0, Windows NT 5.1, 360SE)",
		"Mozilla/4.0 (compatible, MSIE 8.0, Windows NT 6.0, Trident/4.0)",
		"Mozilla/5.0 (compatible, MSIE 9.0, Windows NT 6.1, Trident/5.0,",
		"Opera/9.80 (Windows NT 6.1, U, en) Presto/2.8.131 Version/11.11",
		"Mozilla/4.0 (compatible, MSIE 7.0, Windows NT 5.1, TencentTraveler 4.0)",
		"Mozilla/5.0 (Windows, U, Windows NT 6.1, en-us) AppleWebKit/534.50 (KHTML, like Gecko) Version/5.1 Safari/534.50",
		"Mozilla/5.0 (Macintosh, Intel Mac OS X 10_7_0) AppleWebKit/535.11 (KHTML, like Gecko) Chrome/17.0.963.56 Safari/535.11",
		"Mozilla/5.0 (Macintosh, U, Intel Mac OS X 10_6_8, en-us) AppleWebKit/534.50 (KHTML, like Gecko) Version/5.1 Safari/534.50",
		"Mozilla/5.0 (Linux, U, Android 3.0, en-us, Xoom Build/HRI39) AppleWebKit/534.13 (KHTML, like Gecko) Version/4.0 Safari/534.13",
		"Mozilla/5.0 (iPad, U, CPU OS 4_3_3 like Mac OS X, en-us) AppleWebKit/533.17.9 (KHTML, like Gecko) Version/5.0.2 Mobile/8J2 Safari/6533.18.5",
		"Mozilla/4.0 (compatible, MSIE 7.0, Windows NT 5.1, Trident/4.0, SE 2.X MetaSr 1.0, SE 2.X MetaSr 1.0, .NET CLR 2.0.50727, SE 2.X MetaSr 1.0)",
		"Mozilla/5.0 (iPhone, U, CPU iPhone OS 4_3_3 like Mac OS X, en-us) AppleWebKit/533.17.9 (KHTML, like Gecko) Version/5.0.2 Mobile/8J2 Safari/6533.18.5",
		"MQQBrowser/26 Mozilla/5.0 (Linux, U, Android 2.3.7, zh-cn, MB200 Build/GRJ22, CyanogenMod-7) AppleWebKit/533.1 (KHTML, like Gecko) Version/4.0 Mobile Safari/533.1",
	}
	// atagRegExp = regexp.MustCompile(`<a[^>]+[(href)|(HREF)]\s*\t*\n*=\s*\t*\n*[(".+")|('.+')][^>]*>[^<]*</a>`) //以Must前缀的方法或函数都是必须保证一定能执行成功的,否则将引发一次panic
	detailsExp = regexp.MustCompile(`<a[^>]href=['"](.*)((/video/play.*?)-(\d+).*?)["'][.\s\S]*?src=["'](.*?)(/\d.*?)["']\s`)
	listExp    = regexp.MustCompile(`<a[^>]href=['"](.*list.*?)["']`)
	itemExp    = regexp.MustCompile(`id="d_picTit">(.*)</span>`)
	excutedMap = sync.Map{} //爬取过的详情页
	listRecord = sync.Map{} //已经爬取过的列表页
)

type Data struct {
	domainCat string //代号 tuantuan
	domain    string //host
	url       string //视频地址 - host
	oid       string //视频id
	name      string
	picDomain string    //图片host
	pic       string    //图片地址 - host
	date      time.Time //爬取时间
}

func main() {
	Spy(host)
	// for url := range urlChannel {
	// 	fmt.Println("routines num = ", runtime.NumGoroutine(), " chan len = ", len(urlChannel))
	// 	go Spy(url)
	// }
}

func Spy(url string) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("[E]", err)
		}
	}()

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", GetRandomUserAgent())
	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		fmt.Errorf("get请求%s返回错误:%s", url, err)
		return
	}
	if res.StatusCode == 200 {
		body := res.Body
		defer body.Close()
		bodyByte, _ := ioutil.ReadAll(body)
		resStr := string(bodyByte)

		//列表内数据
		details := detailsExp.FindAllStringSubmatch(resStr, -1)
		for _, detail := range details {
			var domain = detail[1]
			var url = detail[2]
			var oid = detail[4]
			var picDomain = detail[5]
			var pic = detail[6]

			var data = &Data{
				domain:    domain,
				url:       url,
				picDomain: picDomain,
				pic:       pic,
				oid:       oid,
			}
			_, ok := excutedMap.LoadOrStore(domain+url, data)
			if ok && len(domain+url) != 0 {
				continue
			}
			if len(domain) == 0 || len(url) == 0 || len(picDomain) == 0 || len(pic) == 0 || len(oid) == 0 {
				fmt.Println("detailsExp匹配失败:", domain+url)
				continue
			}
			urlChannel <- domain + url
		}

		//爬取列表页
		lists := listExp.FindAllStringSubmatch(resStr, -1)
		for _, list := range lists {
			var url = list[1]
			if len(url) == 0 {
				continue
			}
			//跳过已经爬取过的list列表页
			if _, ok := listRecord.LoadOrStore(url, true); ok {
				continue
			}
			urlChannel <- url
		}

		//爬取详情页
		items := itemExp.FindAllStringSubmatch(resStr, -1)
		for _, item := range items {
			var name = item[1]
			if len(name) == 0 {
				continue
			}

			//补全 视频名称与链接地址

		}
	}
}

func GetRandomUserAgent() string {
	return userAgent[r.Intn(len(userAgent))]
}
