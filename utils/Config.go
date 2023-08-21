package utils

import (
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"path"
	"strings"
)

func NewConfig() *Config {
	wd, err := os.Getwd()
	if err != nil {
		return nil
	}
	configFile := path.Join(wd, "config.json")
	file, err := os.ReadFile(configFile)
	if err != nil {
		log.Println("未找到配置文件")
		return nil
	}
	if err != nil {
		return nil
	}
	config := Config{}
	err = json.Unmarshal(file, &config)
	if err != nil {
		return nil
	}
	return &config
}

func StringToInt(i string) int {
	var result int
	for _, v := range i {
		result = result*10 + int(v-'0')
	}
	return result
}

func WriteConfig(config *Config) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	configFile := path.Join(wd, "config.json")
	file, _ := json.Marshal(config)
	err = os.WriteFile(configFile, file, 0666)
	if err != nil {
		return err
	}
	return nil
}

func (config *Config) GetConfigPage(watcher *fsnotify.Watcher) string {
	return fmt.Sprintf(`
<style>
</style>	
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0-alpha1/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-GLhlTQ8iRABdZLl6O3oVMWSktQOp6b7In1Zl3/Jr59b6EGGoI1aFkw7cmDA6j6gD" crossorigin="anonymous">
<div class="row m-4">
<div  class="col-6 card mr-4">
    <h3 class="mt-4 mb-4">服务器配置</h3>
    <form action="/config" method="post" enctype="application/json">
            <label class="form-label">
                服务器IP:
                <input type="text" value="%s"  class="form-control" name="targetHost" placeholder="输入同步服务器IP">
            </label>
            <label class="form-label">
                服务器端口:
                <input type="number"  class="form-control" value="%d" name="targetPort" placeholder="输入服务器端口号">
            </label>
		<p>
			<label class="form-label">
			同步文件夹:
			<input type="text" value="%s"  class="form-control" name="targetDir" placeholder="输入同步文件夹">
			</label>
			<label>
				忽略文件:
				<input type="text" value="%s"  class="form-control" name="ignored" placeholder="输入忽略文件，以,号分割">
			</label>
		</p>
		<input type="submit" class="form-control" value="保存">
    </form>
</div>
<div class="operate card p-3 col-6 ml-2" >
    <h3 class="mt-4 mb-4">操作</h3>
    <p>监听文件夹列表：%v</p>
    <div style="width: 300px">
		<button id="startBtn" onclick="startWatcher()" class="btn btn-primary">启动监听</button>
    	<button onclick="syncLocal()" class="btn btn-primary">同步本地到远程</button>
		<a href="/refresh" class="btn btn-primary">刷新监听</a>
	</div>
    <div id="result" style="width: 500px; height: 300px"></div>
    <!--    <button>同步文件</button>-->
    <!--    <button>同步远程文件</button>-->
</div>
</div>
<script>
    window.onload = function (){
        startBtn.style.display = %v ? 'none' : 'block';
    }
    function startWatcher(){
        window.location.href = "/start";
    }
    function syncLocal(){
        const result = document.getElementById("result");
        result.style.display = "block"
        fetch("/sync?action=remote", {
            method: 'put',
        }).catch(err=>{
            result.innerText = err;
        }).then(res=>{
            return res.json()
        }).then(msg=>{
            result.innerText = msg['message']
        })
    }
</script>
`, config.TargetHost, config.TargetPort, config.TargetDir, config.Ignored, watcher.WatchList(), len(watcher.WatchList()) > 0)
}

type Config struct {
	Listen     int    `json:"listen"`
	TargetDir  string `json:"targetDir"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
	Ignored    string `json:"ignored"` // 以逗号分隔的忽略文件列表
}

func (config *Config) GetIgnoreList() []string {
	return split(config.Ignored, ",")
}

func split(ignored string, s string) []string {
	result := strings.Split(ignored, s)
	return result
}
