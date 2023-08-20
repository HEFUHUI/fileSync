package main

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	formatTargetDir()
}

var (
	targetDir  = "./target"
	targetHost = "127.0.0.1"
	targetPort = "8080"
	watcher    *fsnotify.Watcher
)

func main() {
	engine := gin.Default()
	engine.GET("/", func(context *gin.Context) {
		go func() {
			err := watchDir(targetDir)
			if err != nil {
				log.Println(err)
			}
		}()
		context.JSON(200, gin.H{
			"message": "ok",
		})
	})
	engine.POST("/sync", func(context *gin.Context) {
		_ = watcher.Remove(targetDir)
		action := context.Query("action")
		fileName := context.Query("fileName")
		fileName = strings.ReplaceAll(fileName, "1100", "\\")
		if action == "delete" {
			err := removeFile(path.Join(targetDir, fileName))
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
			if err := os.Rename(path.Join(targetDir, fileName), path.Join(targetDir, newFileName)); err != nil {
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
			err := os.Mkdir(path.Join(targetDir, fileName), os.ModePerm)
			log.Printf("mkdir %s error: %s\n", fileName, err.Error())
			if err != nil {
				context.JSON(500, gin.H{
					"message": err.Error(),
				})
				return
			}
			context.JSON(200, gin.H{
				"message": "ok",
			})
			return
		} else if action == "upload" {
			log.Printf("upload file %s\n", fileName)
			targetFileName := path.Join(targetDir, fileName)
			bytes, _ := io.ReadAll(context.Request.Body)
			err := os.WriteFile(targetFileName, bytes, os.ModePerm)
			if err != nil {
				context.JSON(500, gin.H{
					"message": err.Error(),
				})
				return
			}
		}
		_ = watcher.Add(targetDir)
		context.JSON(200, gin.H{
			"message": "ok",
		})
	})
	log.Fatal(engine.Run(":8081"))
}

func removeFile(fileName string) error {
	err := os.Remove(path.Join(fileName))
	if err != nil {
		return err
	}
	return nil
}
func watchDir(dir string) error {
	err := watcher.Add(dir)
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
					log.Println(err)
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
						log.Println(err)
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
					log.Println(err)
				}
			} else if event.Op&fsnotify.Rename == fsnotify.Rename {
				log.Printf("rename file: %s\n", event.Name)
				// 获取改名后的文件名
				if err := sendHttpRequest(RequestModel{
					Action:   "delete",
					FileName: event.Name,
				}); err != nil {
					log.Println(err)
				}
			}
		case err := <-watcher.Errors:
			log.Println("error:", err)
		}
	}
}

func sendHttpRequest(req RequestModel) error {
	var err error
	var response *http.Response
	req.FileName = strings.ReplaceAll(req.FileName, targetDir, "")
	if strings.HasPrefix(req.FileName, "\\") {
		req.FileName = strings.Replace(req.FileName, "\\", "", 1)
	}
	// 将/替换为\
	req.FileName = strings.ReplaceAll(req.FileName, "\\", "1100")
	url := fmt.Sprintf("http://%s:%s/sync?action=%s&fileName=%s", targetHost, targetPort, req.Action, req.FileName)
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
	log.Println("response status code is", response.StatusCode)
	if response.StatusCode != 200 {
		contentType := response.Header["Content-Type"]
		if len(contentType) > 0 && strings.Contains(contentType[0], "application/json") {
			var msg string
			_, _ = fmt.Fscanf(response.Body, `{"message":"%s"}`, &msg)
			return fmt.Errorf("response status code is %d, msg: %s", response.StatusCode, msg)
		}
		return fmt.Errorf("response status code is %d", response.StatusCode)
	} else {
		log.Println("response status code is 200")
	}
	return nil
}

func formatTargetDir() {
	if strings.HasPrefix(targetDir, "./") {
		targetDir = strings.ReplaceAll(targetDir, "./", "")
	}
	if strings.HasPrefix(targetDir, "/") {
		targetDir = strings.Replace(targetDir, "/", "", 1)
	}
}

type RequestModel struct {
	Action   string   `json:"action"`
	Body     *os.File `json:"body"`
	FileName string   `json:"fileName"`
}
