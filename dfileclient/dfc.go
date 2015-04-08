// dfileclient project main.go
package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

var basedir, remoteAddr, stype, estring string

//写文件到网络
func fileToTcp(w io.Writer, r io.Reader) {
	io.Copy(w, r)
}

//判断文件是否存在
func fExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

//创建一个连接，如果连接不上，等待3秒后重试
func getConn() net.Conn {
	var c net.Conn
	var e error
	for {
		c, e = net.Dial("tcp", remoteAddr)
		if e != nil {
			lg.Println("can't not dial. wait 3 second to retry...")
			time.Sleep(time.Second * 3)
		} else {
			lg.Println("New connection estabinished: ", c.LocalAddr(), c.RemoteAddr())
			break
		}

	}
	return c
}

//转发送过来的文件进行处理，返回处理过的文件名
func fileProcess(infile string) string {
	outfile := infile

	exes := strings.Replace(estring, `#i#`, infile, -1) //#i#替换成真正文件名
	cmd := new(exec.Cmd)
	tslice := strings.Split(exes, " ")
	cmd.Path = tslice[0]
	cmd.Args = tslice
	lg.Println("The command running: ", cmd.Args)
	_, e := cmd.Output()
	if e != nil {
		lg.Println("execute os command error: ", e)
	} else {
		lg.Println("outfile will be change to:", tslice[len(tslice)-1])
		outfile = tslice[len(tslice)-1]
	}
	return outfile
}

//根据客户配置的类型，判断是否需要处理，-e 参数可以指定处理的方法。
func openFile(fn string) *os.File {
	var f *os.File

	switch stype {
	case "audio":
		lg.Println("audio type processing...")
		newName := fileProcess(fn)
		f, _ = os.Open(newName)
	case "any":
		f, _ = os.Open(fn)
	default:
		f, _ = os.Open(fn)
	}
	return f
}

//处理接到的文件名，查找请求并反馈
func clientConn(needNewChan chan bool) {
	c := getConn()
	defer c.Close()
	reader := bufio.NewReader(c)

	fndata, _, _ := reader.ReadLine()
	needNewChan <- true //to make next connection
	os.Chdir(basedir)
	filename := basedir + string(fndata)
	lg.Println("The file name is:", filename)

	if !fExist(filename) {
		lg.Println("The file can't find, socket close")
		c.Write([]byte("0"))

	} else {
		lg.Println("found file!")

		c.Write([]byte("1"))
		lg.Println("write file to tcp!")
		f := openFile(filename)
		fileToTcp(c, f)
		f.Close()

	}
	c.Close()
	lg.Println("socket close:", c.LocalAddr(), c.RemoteAddr())
}

var lg = log.New(os.Stdout, " ", log.LstdFlags|log.Llongfile)
var serveType string

//处理命令行参数
func parseFlag() {
	flag.StringVar(&basedir, "b", `.`, "-b = \"/home\"")
	flag.StringVar(&remoteAddr, "r", `127.0.0.1:8612`, "-host=\"127.0.0.1:8612\"")
	flag.StringVar(&stype, "t", "any", "-t=\"audio\"  \"Support audio only\"")
	flag.StringVar(&estring, "e", " ", `sox.exe #i# #i#.mp3,where "#i# is the variable will replace with the file`)
	flag.Parse()
}

func main() {
	parseFlag()
	needNewChan := make(chan bool)
	go clientConn(needNewChan)
	for {
		select {
		case <-needNewChan:
			go clientConn(needNewChan)
		}
	}
}
