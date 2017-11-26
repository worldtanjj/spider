package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/axgle/mahonia"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	host       = "http://www.sihu888.com/"
	urlChannel = make(chan string, 20)
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
	listExp    = regexp.MustCompile(`<a[^>]href=['"](.*?video/list.*?)["']`)
	itemExp    = regexp.MustCompile(`id="d_picTit">(.*)</span>[\s\S]+</iframe>.+</script>[\s\S]+?<iframe.+src=["'](.+?)["']`)
	oidExp     = regexp.MustCompile(`play-(\d+)`)
	excutedMap = sync.Map{} //爬取过的详情页
	listRecord = sync.Map{} //已经爬取过的列表页
	session    *mgo.Session
	tasks      sync.WaitGroup
)

type Data struct {
	DomainCat string    `bson:"domainCat"` //代号 tuantuan
	Domain    string    `bson:"domain"`    //host
	URL       string    `bson:"url"`       //视频地址 - host
	Oid       string    `bson:"oid"`       //视频id
	Name      string    `bson:"name"`
	PicDomain string    `bson:"picDomain"` //图片host
	Pic       string    `bson:"pic"`       //图片地址 - host
	Date      time.Time `bson:"date"`      //爬取时间
}

func init() {
	var err error
	session, err = mgo.Dial("mongodb://localhost/mydb")
	if err != nil {
		panic(err)
	}
}
func main() {
	defer session.Close()
	// Optional. Switch the session to a monotonic behavior.
	session.SetMode(mgo.Monotonic, true)

	tasks.Add(1)
	go func() {
		go Spy(host)
		for url := range urlChannel {
			tasks.Add(1)
			go Spy(url)
		}
	}()
	tasks.Wait()
	close(urlChannel)
	fmt.Println("执行完毕")
	//把爬取过的detail地址与list地址写入io
	WriteInfoTxt()
}

func Spy(url string) {
	// defer func() {
	// 	if err := recover(); err != nil {
	// 		log.Println("[E]", err)
	// 	}
	// }()
	defer tasks.Done()

	fmt.Println("正在爬取: ", url, "chan len =:", len(urlChannel))
	c := session.DB("mydb").C("spider1")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("NewRequest 失败", url, err)
		return
	}
	//req.Header.Set("User-Agent", GetRandomUserAgent())
	// client := http.DefaultClient
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("get请求%s返回错误:%s", url, err)
		return
	}
	body := res.Body
	defer body.Close()
	if res.StatusCode == 200 {
		bodyByte, err := ioutil.ReadAll(body)
		if err != nil {
			fmt.Println("==============================读取Body失败:", err.Error())
			return
		}
		fmt.Println("Read后: ", url, "body长度:", len(bodyByte))
		resStr := string(bodyByte)
		resStr = ConvertToString(resStr, "gbk", "utf-8")
		if len(resStr) == 0 {
			fmt.Println("===========================内容为空: 爬取页面: ", url)
			// f, _ := os.OpenFile("./miss.txt", os.O_WRONLY|os.O_APPEND, 0666)
			// f.WriteString(url + " 爬取失败")
			// f.Close()
			// time.Sleep(5 * time.Second)
			// Spy(url)
			return
		}
		//视频数据
		details := detailsExp.FindAllStringSubmatch(resStr, -1)
		for _, detail := range details {
			var domain = detail[1]
			var detailUrl = detail[1] + detail[2] //完整的详情页地址
			var oid = detail[4]
			var picDomain = detail[5]
			var pic = detail[6]

			var data = &Data{
				Domain:    domain,
				DomainCat: "tuantuan",
				PicDomain: picDomain,
				Pic:       pic,
				Oid:       oid,
			}
			_, ok := excutedMap.LoadOrStore(detailUrl, data)
			if ok && len(detailUrl) != 0 {
				continue
			}
			if len(detailUrl) == 0 || len(picDomain) == 0 || len(pic) == 0 || len(oid) == 0 {
				continue
			}
			urlChannel <- detailUrl
		}

		//爬取列表页
		lists := listExp.FindAllStringSubmatch(resStr, -1)
		for _, list := range lists {
			var listUrl = list[1]
			if len(listUrl) == 0 {
				continue
			}
			//跳过已经爬取过的list列表页
			if _, ok := listRecord.LoadOrStore(listUrl, true); ok {
				continue
			}
			urlChannel <- listUrl
		}

		//爬取详情页
		items := itemExp.FindAllStringSubmatch(resStr, -1)
		for _, item := range items {
			var name = item[1]
			var videoUrl = item[2]
			if len(name) == 0 || len(videoUrl) == 0 {
				continue
			}

			//补全 视频名称与链接地址
			if val, ok := excutedMap.Load(url); ok {
				var data = val.(*Data)
				data.Name = name
				data.URL = videoUrl
				data.Date = time.Now()

				//把url中oid正则出来
				allMatch := oidExp.FindAllStringSubmatch(url, -1)
				var oid = allMatch[0][1]
				if len(oid) == 0 {
					fmt.Println("匹配oid失败,url: ", url)
					continue
				}
				//补全信息后添加到mongo中
				var result = Data{}
				c.Find(bson.M{"oid": oid}).One(&result)
				if len(result.Oid) == 0 {
					err = c.Insert(data)
					if err != nil {
						log.Fatal(err)
					}
				}
			}
		}
		fmt.Println("正则匹配完毕,url:", url)
		return
	}
	fmt.Printf("=============================请求%s相应码为:%d", url, res.StatusCode)
	return
}

func GetRandomUserAgent() string {
	return userAgent[r.Intn(len(userAgent))]
}
func ConvertToString(src string, srcCode string, tagCode string) string {
	srcCoder := mahonia.NewDecoder(srcCode)
	srcResult := srcCoder.ConvertString(src)
	tagCoder := mahonia.NewDecoder(tagCode)
	result := tagCoder.ConvertString(src)
	_, cdata, _ := tagCoder.Translate([]byte(srcResult), true)
	result = string(cdata)
	return result
}
func WriteInfoTxt() {
	var detailFile = "./details.txt"
	var listFile = "./lists.txt"
	fileDetail, err := os.Create(detailFile)
	if err != nil {
		fmt.Println(detailFile, err)
		return
	}
	fileList, err := os.Create(listFile)
	if err != nil {
		fmt.Println(detailFile, err)
		return
	}
	defer fileDetail.Close()
	defer fileList.Close()

	excutedMap.Range(func(k, v interface{}) bool {
		fileDetail.WriteString(k.(string) + "\r\n")
		return true
	})
	listRecord.Range(func(k, v interface{}) bool {
		fileList.WriteString(k.(string) + "\r\n")
		return true
	})
}
