package main

import (
	"fileSync/utils"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

const (
	targetDir  = "./target"
	targetHost = "127.0.0.1"
	targetPort = 8081
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	config = utils.NewConfig()
	if config == nil {
		config = new(utils.Config)
		config.TargetDir = targetDir
		config.TargetHost = targetHost
		config.TargetPort = targetPort
		config.Listen = 6789
	}
	formatTargetDir()
}

var (
	config  *utils.Config
	watcher *fsnotify.Watcher
	reload  = make(chan struct{}, 10)
)

func main() {
	engine := gin.Default()
	gin.SetMode(gin.ReleaseMode)
	engine.Use(gin.Recovery())
	engine.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	}))
	if _, err := os.Stat(config.TargetDir); err != nil {
		log.Println("目标文件夹不存在, 创建文件夹: ", config.TargetDir)
		_ = os.MkdirAll(config.TargetDir, os.ModePerm)
	}
	log.Println("开始监听: ", config.TargetDir)
	go func() {
		err := watchDir(config.TargetDir)
		if err != nil {
			log.Println(err)
		}
	}()
	engine.GET("/refresh", func(context *gin.Context) {
		_ = reloadWatch(config.TargetDir)
		context.Redirect(302, "/")
	})
	engine.POST("/config", func(context *gin.Context) {
		config.TargetHost = context.DefaultPostForm("targetHost", config.TargetHost)
		targetPort := context.DefaultPostForm("targetPort", fmt.Sprintf("%d", targetPort))
		config.TargetPort = utils.StringToInt(targetPort)
		config.Listen = utils.StringToInt(context.DefaultPostForm("listen",
			fmt.Sprintf("%d", config.Listen)))
		config.TargetDir = context.PostForm("targetDir")
		config.Ignored = strings.Trim(context.PostForm("ignored"), ",")
		formatTargetDir()
		for len(watcher.WatchList()) > 0 {
			_ = watcher.Remove(watcher.WatchList()[0])
		}
		_ = reloadWatch(config.TargetDir)
		// 写入配置文件
		if err := utils.WriteConfig(config); err != nil {
			context.JSON(500, gin.H{
				"message": err.Error(),
			})
			return
		}
		context.Redirect(302, "/")
	})
	engine.GET("/start", func(context *gin.Context) {
		go func() {
			err := watchDir(config.TargetDir)
			if err != nil {
				log.Println(err)
			}
		}()
		context.Redirect(302, "/")
	})
	engine.GET("/", func(context *gin.Context) {
		context.Header("Content-Type", "text/html; charset=utf-8")
		_, err := context.Writer.WriteString(config.GetConfigPage(watcher))
		if err != nil {
			context.JSON(500, gin.H{
				"message": err.Error(),
			})
			return
		}
	})
	engine.PUT("/sync", func(context *gin.Context) {
		//手动同步文件夹
		action := context.Query("action")
		if action == "" {
			context.JSON(500, &gin.H{
				"message": "请提供操作",
			})
			return
		}
		switch action {
		case "remote":
			err := copyRemote()
			if err != nil {
				context.JSON(500, &gin.H{
					"message": err.Error(),
				})
			}
		}
		context.JSON(200, &gin.H{
			"message": "ok",
		})
	})
	engine.POST("/sync", func(context *gin.Context) {
		removeAllWatch()
		action := context.Query("action")
		fileName := context.Query("fileName")
		fileName = strings.ReplaceAll(fileName, "1100", "/")
		if action == "delete" {
			err := removeFile(path.Join(config.TargetDir, fileName))
			if err != nil {
				log.Printf("remove file %s error: %s\n", fileName, err.Error())
				context.JSON(500, gin.H{
					"message": err.Error(),
				})
				return
			}
			context.JSON(200, gin.H{
				"message": "ok",
			})
			return
		} else if action == "rename" {
			newFileName := context.Query("newFileName")
			if err := os.Rename(path.Join(config.TargetDir, fileName), path.Join(config.TargetDir, newFileName)); err != nil {
				log.Printf("rename file %s error: %s\n", fileName, err.Error())
				context.JSON(500, gin.H{
					"message": err.Error(),
				})
				return
			}
			context.JSON(200, gin.H{
				"message": "ok",
			})
			return
		} else if action == "mkdir" {
			err := os.Mkdir(path.Join(config.TargetDir, fileName), os.ModePerm)
			if err != nil {
				log.Printf("mkdir %s error: %s\n", fileName, err.Error())
				context.JSON(500, gin.H{
					"message": err.Error(),
				})
				return
			}
			context.JSON(200, gin.H{
				"message": "ok",
			})
			reload <- struct{}{}
			return
		} else if action == "upload" {
			log.Printf("upload file %s\n", fileName)
			targetFileName := path.Join(config.TargetDir, fileName)
			dir := path.Dir(targetFileName)
			// 判断是否是存在
			if _, err := os.Stat(dir); err != nil {
				if os.IsNotExist(err) {
					// 不存在, 创建文件夹
					err := os.MkdirAll(dir, os.ModePerm)
					if err != nil {
						log.Println(err)
					}
				} else {
					log.Println(err)
				}
			}
			bytes, _ := io.ReadAll(context.Request.Body)
			err := os.WriteFile(targetFileName, bytes, os.ModePerm)
			if err != nil {
				context.JSON(500, gin.H{
					"message": err.Error(),
				})
				return
			}
		}
		err := reloadWatch(config.TargetDir)
		if err != nil {
			context.JSON(500, gin.H{
				"message": err.Error(),
			})
			return
		}
		context.JSON(200, gin.H{
			"message": "ok",
		})
	})
	log.Println("监听端口: ", config.Listen)
	log.Fatal(engine.Run(fmt.Sprintf(":%d", config.Listen)))
}

func copyRemote() error {
	return sendDir(config.TargetDir)
}

func sendDir(dir string) error {
	dirFileList, err := os.ReadDir(dir)
	if err != nil {
		log.Println("同步失败：" + err.Error())
		return err
	}
	for _, item := range dirFileList {
		if matchIgnore(item.Name()) {
			continue
		}
		if item.IsDir() {
			err := sendDir(path.Join(dir, item.Name()))
			if err != nil {
				log.Printf("文件夹：%s, 同步失败：%s", item.Name(), err.Error())
			}
		} else {
			file, err := os.OpenFile(path.Join(dir, item.Name()), os.O_RDONLY, 0666)
			if err != nil {
				log.Printf("文件: %s,读取失败: %s", item.Name(), err.Error())
			}
			if err = sendHttpRequest(RequestModel{
				Action:   "upload",
				Body:     file,
				FileName: path.Join(dir, item.Name()),
			}); err != nil {
				log.Printf("文件: %s,同步失败: %s", item.Name(), err.Error())
			}
		}
	}
	return nil
}

func removeFile(fileName string) error {
	err := os.Remove(path.Join(fileName))
	if err != nil {
		return err
	}
	return nil
}

func reloadWatch(dir string) error {
	removeAllWatch()
	err := watcher.Add(dir)
	subDir := utils.GetSubDir(dir)
	for _, newDir := range subDir {
		if matchIgnore(newDir) {
			log.Println("忽略文件夹: ", newDir)
			continue
		}
		err = watcher.Add(newDir)
		if err != nil {
			log.Printf("文件夹 %s 监听添加失败: %v", newDir, err)
		}
	}
	return err
}

func removeAllWatch() {
	for len(watcher.WatchList()) > 0 {
		_ = watcher.Remove(watcher.WatchList()[0])
	}
}

func watchDir(dir string) error {
	err := reloadWatch(dir)
	if err != nil {
		return err
	}
	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Printf("modified file: %s\n", event.Name)
				if fileInfo, err := os.Stat(event.Name); err != nil || fileInfo.IsDir() {
					continue
				}
				file, _ := os.OpenFile(event.Name, os.O_RDONLY, os.ModePerm)
				err := sendHttpRequest(RequestModel{
					Action:   "upload",
					FileName: event.Name,
					Body:     file,
				})
				if err != nil {
					log.Println(err)
				}
			} else if event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("create file: %s\n", event.Name)
				fileInfo, err := os.Stat(event.Name)
				if err != nil {
					log.Println("create file error:", err)
					continue
				}
				if fileInfo.IsDir() {
					if err := sendHttpRequest(RequestModel{
						Action:   "mkdir",
						FileName: event.Name,
					}); err != nil {
						log.Println(err)
					}
					if err = watcher.Add(event.Name); err != nil {
						log.Println("watcher add error:", err)
					} else {
						log.Println("add watch", path.Join(targetDir, event.Name))
					}
				} else {
					file, _ := os.OpenFile(event.Name, os.O_RDONLY, os.ModePerm)
					if err := sendHttpRequest(RequestModel{
						Action:   "upload",
						FileName: event.Name,
						Body:     file,
					}); err != nil {
						log.Println("create remote file error:", err)
					}
				}
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				log.Printf("remove file: %s\n", event.Name)
				// 如果在监听列表中的，说明是文件夹, 需要移除监听
				// 判断字符串是否在切片中
				for _, watchName := range watcher.WatchList() {
					if event.Name == watchName {
						log.Println("remove watch", watchName)
						_ = watcher.Remove(watchName)
						break
					}
				}
				if err := sendHttpRequest(RequestModel{
					Action:   "delete",
					FileName: event.Name,
				}); err != nil {
					log.Println("remove remote file error:", err)
				}
			} else if event.Op&fsnotify.Rename == fsnotify.Rename {
				log.Printf("rename file: %s\n", event.Name)
				// 获取改名后的文件名
				if err := sendHttpRequest(RequestModel{
					Action:   "delete",
					FileName: event.Name,
				}); err != nil {
					log.Println("remove remote file error:", err)
				}
			}
		case err := <-watcher.Errors:
			log.Println("error:", err)
		case <-time.After(time.Second * 1):
			if len(watcher.WatchList()) < 1 {
				err := reloadWatch(dir)
				if err != nil {
					log.Println("reload watch error:", err)
				}
			}
		}
	}
}

func sendHttpRequest(req RequestModel) error {
	if matchIgnore(req.FileName) {
		log.Println("忽略文件: ", req.FileName)
		return nil
	}
	var err error
	var response *http.Response
	req.FileName = req.FileName[len(config.TargetDir):]
	if strings.HasPrefix(req.FileName, "\\") || strings.HasPrefix(req.FileName, "/") {
		req.FileName = req.FileName[1:]
	}
	req.FileName = strings.ReplaceAll(req.FileName, "\\", "1100")
	req.FileName = strings.ReplaceAll(req.FileName, "/", "1100")
	req.FileName = strings.ReplaceAll(req.FileName, " ", "1200")
	url := fmt.Sprintf("http://%s:%d/sync?action=%s&fileName=%s", config.TargetHost, config.TargetPort, req.Action, req.FileName)
	if req.Body != nil {
		response, err = http.Post(url, "binary/octet-stream", req.Body)
	} else {
		response, err = http.Post(url, "application/json", nil)
	}
	// URL 编码
	if err != nil {
		log.Println(err)
		return err
	}
	if response.StatusCode != 200 {
		contentType := response.Header["Content-Type"]
		if len(contentType) > 0 && strings.Contains(contentType[0], "application/json") {
			var msg string
			_, _ = fmt.Fscanf(response.Body, `{"message":"%s"}`, &msg)
			return fmt.Errorf("response status code is %d, msg: %s", response.StatusCode, msg)
		}
		return fmt.Errorf("response status code is %d", response.StatusCode)
	} else {
		log.Printf("file %s sync success", req.FileName)
	}
	return nil
}

func formatTargetDir() {
	config.TargetDir = path.Clean(config.TargetDir)
}

type RequestModel struct {
	Action   string   `json:"action"`
	Body     *os.File `json:"body"`
	FileName string   `json:"fileName"`
}

func matchIgnore(fileName string) bool {
	name := path.Base(fileName)
	for _, ignoreItem := range config.GetIgnoreList() {
		// 如果item是以/结尾的，说明是文件夹
		if strings.HasSuffix(ignoreItem, "/") {
			ignoreItem = strings.ReplaceAll(ignoreItem, "/", "")
			// 如果文件名包含ignoreItem，说明是文件夹，并且不是以ignoreItem结尾的
			if strings.Contains(name, ignoreItem) {
				return true
			}
		}
		// 如果item包含*，说明是通配符
		if strings.Contains(ignoreItem, "*") {
			ignoreItem = strings.ReplaceAll(ignoreItem, "*", "")
			if strings.HasPrefix(name, ignoreItem) || strings.HasSuffix(name, ignoreItem) {
				return true
			}
		}
	}
	return false
}
