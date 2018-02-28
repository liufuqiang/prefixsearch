package main

import (
	"flag"
	"fmt"
	"net/url"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/fvbock/trie"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/file"
)

var tree *trie.Trie

type Pair struct {
	Key   string
	Value int
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value > p[j].Value }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

var action string

func init() {
	flag.StringVar(&action, "action", "", "dump: dump data to file,load: load data to memery")
}

var queryList = make(chan string, 5000)
var once sync.Once

func main() {
	flag.Parse()
	go doAdd()

	if action == "load" {
		var err error
		tree, err = trie.LoadFromFile("./data.db")
		if err != nil {
			tree = trie.NewTrie()
		}
	} else {
		tree = trie.NewTrie()
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.GET("/search", search)
	router.GET("/dump", dump)
	router.GET("/status", status)
	router.GET("/add", add)
	router.GET("/reset", reset)
	router.Run(":18080")
}

func dump(c *gin.Context) {
	fmt.Println("dump to file:", tree.DumpToFile("./data.db"))
}

func doAdd() {
	for {
		q := <-queryList
		go func(q string) {
			tree.Add(reverUrl(q))
		}(q)
	}
}

func reset(c *gin.Context) {
	tree = trie.NewTrie()
	runtime.GC()
}

func add(c *gin.Context) {
	q, _ := c.GetQuery("q")
	if q == "" || len(q) < 10 {
		return
	}

	go func() {
		queryList <- q
	}()
}

func status(c *gin.Context) {
	c.String(200, "%s", "ok\n")
}

func search(c *gin.Context) {
	q := c.Query("q")
	n := c.Query("n")
	js := c.Query("json")
	if js == "" {
		js = "0"
	}

	if n == "" {
		n = "10"
	}

	limit, err := strconv.Atoi(n)
	if err != nil {
		limit = 10
	}

	subquery := getSubQuery(q)

	urls := tree.PrefixMembers(reverUrl(subquery))

	var urlList PairList

	for _, u := range urls {
		_url := reverUrl(u.Value)
		matched, err := regexp.MatchString(q, _url)
		if matched == true && err == nil {
			urlList = append(urlList, Pair{Key: _url, Value: int(u.Count)})
		}
	}

	sort.Sort(urlList)

	var totalPV int
	var ret []string
	for _, item := range urlList {
		totalPV += item.Value
		ret = append(ret, fmt.Sprintf("%s\t%d", item.Key, item.Value))
	}

	totalNum := len(urlList)
	if totalNum < limit {
		limit = totalNum
	}

	if js == "1" {
		c.JSON(200, gin.H{"err_code": 0, "err_msg": "", "totalNum": totalNum, "totalPV": totalPV, "data": urlList[:limit]})
	} else {
		c.String(200, "%s", fmt.Sprintf("Total Num:%d Total PV:%d\n%s", totalNum, totalPV, strings.Join(ret[:limit], "\n")))
	}
}

func getSubQuery(q string) string {
	q = strings.Replace(q, `\.`, ".", -1)
	q = strings.Replace(q, `\?`, "?", 1)

	b := []byte(q)
	for i := 0; i < len(b); i++ {
		if b[i] == byte('[') || b[i] == byte('(') || b[i] == byte('\\') {
			return string(b[:i])
		}
	}
	return q
}

func buildTree() {
	lines, err := file.ToTrimString("./u.txt")
	if err != nil {
		panic(err)
	}

	for _, line := range strings.Split(lines, "\n") {
		line = strings.Trim(line, " ")
		if line == "" {
			continue
		}
		tree.Add(reverUrl(line))
	}
}

func reverUrl(_url string) string {
	u, err := url.Parse(_url)
	if err != nil {
		return _url
	}

	hostname := u.Hostname()
	arr := strings.Split(hostname, ".")
	var ret []string

	for i := len(arr) - 1; i >= 0; i-- {
		ret = append(ret, arr[i])
	}
	u.Host = strings.Join(ret, ".")
	return u.String()
}
