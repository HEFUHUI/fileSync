## 文件同步工具

### 1. 介绍

使用golang实现，分两个服务运行，本地和远程。通过http和fileWatcher实现文件同步。

### 2. 使用说明

#### 2.1 本地服务

##### 2.1.1 启动

```shell
./fileSync 
```

##### 2.1.2 配置文件

```json
{
}
```
##### 2.1.3 命令使用说明

| 参数   | 说明     | 类型   | 是否必填 |
| ------ | -------- | ------ | -------- |
