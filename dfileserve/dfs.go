package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
)

var sockets = make([]net.Conn, 0)
var sLock sync.Mutex

func tcpFileToHTTP(httpWriter http.ResponseWriter, fileReceiver net.Conn) error {
	_, e := io.Copy(httpWriter, fileReceiver)
	lg.Println(fileReceiver.LocalAddr(), fileReceiver.RemoteAddr())
	fileReceiver.Close()
	return e
}

func getFileName(fn string) string {
	return fn
}

//根据返回的reader 判断是否找到了文件
func findFile(fn string) (conn net.Conn, found bool) {
	found = false
	if len(sockets) == 0 {
		lg.Println("no client connected!")
		return nil, found
	}

	fr := getFileReader(fn)

	if fr != nil {
		lg.Println("found file")
		found = true
		return fr, found
	}
	lg.Println("no found")
	return nil, found
}

//扫描客户端判断看看有没有结果
func getFileReader(fn string) net.Conn {
	//lock to copy current sockets.
	sLock.Lock()
	tmpSockets := sockets
	sLock.Unlock()

	c := make(chan net.Conn) //通过chan 接收返回的连接
	cnt := make(chan int, 1) //返回的结果计数器
	maxcnt := len(tmpSockets)
	lg.Println("the length of tmp sockets is :", maxcnt)

	for _, s := range tmpSockets {
		go sendNameGetReader(s, fn, c, cnt)
	}

	//等待所有socket 结束，如果没找到则返一个nil 给filereader
	j := 0
	go func() {
		for {
			<-cnt
			j++
			lg.Println("get ", j, " of ", maxcnt)
			//			maxcnt--
			if j >= maxcnt {
				lg.Println("scan sockets finished!")
				c <- nil //返回nil 给filereader
				break
			}
		}
	}()

	//等待客户端给出结果，可能是找到(get from ↑)，也可能是全都找不到这个文件(get from go routine)
	lg.Println("wait chan ...")
	var fr net.Conn //file reader
	fr = nil
	select {
	case fr = <-c: //这里的filerader 可能得到两个值，一个是找到了文件后返回的正确reader， 另一个是所有遍历所有socket后还是找不到，返回nil， 这个nil在上一层调用会被判为找不到。
		//lg.Println("receive:")
	}

	//将用掉的socket 删除掉
	sLock.Lock()
	lg.Println("befor truncate length is :", len(sockets))
	sockets = sockets[maxcnt:]
	lg.Println("after truncate length is", len(sockets))
	sLock.Unlock()

	return fr
}

//发送文件名，并将结果通过 Conn chan跟 int chan 发给上级调用函数
func sendNameGetReader(s net.Conn, fn string, c chan net.Conn, cnt chan int) error {
	s.Write([]byte(fn + "\r\n")) //
	b := make([]byte, 1)
	lg.Println("wait to read one byte from socket...")

	//tcp 读是阻塞的
	for {
		n, _ := s.Read(b)
		lg.Println("receive:", n, "bytes")
		lg.Println("the byte is", string(b))

		//收到客户端的字符0表示没找到文件
		if string(b) == "0" {
			lg.Println("file no found")
			cnt <- 1
			break
		}

		if n > 0 {
			c <- s
			cnt <- 1
			break
		} else { //收不到客户端的结果了表示收不到
			cnt <- 1
			s.Close()
			break
		}
	}
	lg.Println("go routine,finish")
	return nil
}

//处理发送过来的请求
func ServeAll(rw http.ResponseWriter, r *http.Request) {
	lg.Println("ServeAll function")
	fileName := getFileName(r.URL.Path)
	fileReceiver, found := findFile(fileName)
	if found {
		tcpFileToHTTP(rw, fileReceiver)
	} else {
		http.NotFound(rw, r)
	}
}

var lg = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Llongfile)

func main() {
	l, _ := net.Listen("tcp", ":8612")
	go func() {
		for {
			s, _ := l.Accept()
			defer s.Close()
			sLock.Lock()
			sockets = append(sockets, s)
			lg.Println("New socket coming, current sockets length:", len(sockets))
			sLock.Unlock()

		}
	}()

	h := http.HandlerFunc(ServeAll)
	http.ListenAndServe(":8701", h)
}
