## 1 编译与运行

### 1.1 环境准备

#### 1.1.1 Go环境准备

Go语言环境：**1.24.10**。安装包下载地址：<https://go.dev/dl/>

- **Windows与macOS**环境下，通过下载的安装程序来安装Go。

- **Linux**环境下直接通过安装工具安装：`sudo apt install golang`或者通过一下指令手动安装：

  ```sh
  1. sudo apt-get update
  2. wget https://go.dev/dl/go1.26.2.linux-amd64.tar.gz
  3. sudo tar -xvf go1.26.2.linux-amd64.tar.gz
  4. sudo mv go /usr/local
  5. export GOROOT=/usr/local/go
  6. export GOPATH=$HOME/go
  7. export PATH=$GOPATH/bin:$GOROOT/bin:$PATH
  8. source ~/.profile
  ```

- 安装成功后，运行指令`go version`来查看Go版本号。

#### 1.1.2 Wails环境准备

Wails 有许多安装前需要的常见依赖项：

- Go 1.21+ (macOS 15+ requires Go 1.23.3+)
- NPM (Node 15+)

从 [Node 下载页面](https://nodejs.org/en/download/) 下载 NPM，运行 `npm --version` 进行验证。

Wails平台特定依赖：

- **Windows**: Wails 要求安装 [WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) 运行时。 
- **MacOS**: Wails 要求安装 xcode 命令行工具。 这可以通过运行 `xcode-select --install` 来完成。
- **Linux**: Linux 需要标准的 `gcc` 构建工具以及 `libgtk3` 和 `libwebkit`。

运行 `go install github.com/wailsapp/wails/v2/cmd/wails@latest` 安装 Wails CLI。

成功安装wails后运行 `wails doctor` 将检查您是否安装了正确的依赖项。 如果没有，它会就缺少的内容提供建议以帮助纠正问题。

<small>*(如果您的系统报告缺少 `wails` 命令，请确保您已正确遵循 Go 安装指南。 通常，这意味着您的用户 home 目录中的 `go/bin` 目录不在 `PATH` 环境变量中。 通常情况下还需要关闭并重新打开任何已打开的命令提示符，以便安装程序对环境所做的更改反映在命令提示符中。)*</small>

Wails安装存在疑问，请参考[Wails官方安装说明](https://wails.io/docs/gettingstarted/installation/)进行Wails的安装。

### 1.2 程序编译

- **Windows**环境下，通过执行项目根目录下的批处理文件`build.bat`进行编译。
  - `build.bat`：仅编译stealth-dns.exe可执行文件，该文件可独立运行来进行DNS代理。
  - `build.bat ui`：仅编译stealth-ui.exe可执行文件，该UI文件需要依赖stealth-dns.exe进行DNS代理。
  - `build.bat full`：同时编译stealth-dns.exe和stealth-ui.exe两个执行文件。
- **Linux与macOS**环境下，通过在项目根目录下执行`make`指令进行编译。
  - `make`：仅编译stealth-dns可执行文件，该文件可独立运行来进行DNS代理。
  - `make ui`：仅编译stealth-ui可执行文件，该UI文件需要依赖stealth-dns进行DNS代理。
  - `make full`：同时编译stealth-dns和stealth-ui两个执行文件。
- 编译后的执行文件在`release\`子目录下。
- 配置文件在`release\etc\`子目录下，该目录下的配置文件是复制`etc\`子目录下的配置文件。

### 1.3 配置文件

- config.toml

  ```toml
  # NHP-Agent base config
  # field with (-) does not support dynamic update
  
  # PrivateKeyBase64 (-): agent private key in base64 format.
  # TEEPrivateKeyBase64 (-): TEE private key in base64 format.
  # DefaultCipherScheme: 0: gmsm, 1: curve25519.
  # UserId: specify the user id this agent represents.
  # OrganizationId: specify the organization id this agent represents.
  # LogLevel: 0: silent, 1: error, 2: info, 3: audit, 4: debug, 5: trace.
  PrivateKeyBase64 = "eACeqvhs7AWHIXM+xsKK9cCk31gFinnMGgGI2RuAxUQ="
  DefaultCipherScheme = 1
  UserId = "agent-0"
  OrganizationId = "opennhp.org"
  LogLevel = 4
  # UserData: a customized user entry for flexibility.
  # Its key-value pairs will be send to server along with knock message.
  [UserData]
  "ExampleKey0" = "StringValue"
  "ExampleKey1" = 1
  "ExampleKey2" = true
  ```

  

- server.toml

  ```to
  # list the server peers for the agent under [[Servers]] table
  
  # Hostname: the domain of the server peer. If specified, it overrides the "Ip" field with its first resolved address.
  # Ip: specify the ip address of the server peer
  # Port: specify the port number of this server peer is listening
  # PubKeyBase64: public key of the server peer in base64 format
  # ExpireTime (epoch timestamp in seconds): peer key validation will fail when it expires.
  [[Servers]]
  Hostname = "nhp.opennhp.org"
  Ip = ""
  Port = 62206
  PubKeyBase64 = "T29VClxfuJa7AKA2D+4gBvGXnyGZA35AdqRkdjk49Vs="
  ExpireTime = 1924991999
  ```
  
  

- resource.toml

  ```toml
  # List resources for the agent to knock automatically after launch
  
  # AuthServiceId: id of the authentication and authorization service provider the resource belongs to.
  # ResourceId: id of the resource group.
  # ServerHostname: host name of the NHP server that manages this resource group.
  # ServerIp: ip address of the NHP server  that manages this resource group.
  # ServerPort: port of the NHP server that manages this resource group.
  # NOTE: ServerHostname, ServerIp and ServerPort must match with the Hostname, Ip and Port of the server defined
  # in server.toml in order for the program to locate the correct server peer
  [[Resources]]
  AuthServiceId = "example"
  ResourceId = "demo"
  ServerHostname = "nhp.opennhp.org"
  ServerIp = ""
  ServerPort = 62206
  ```



### 1.4 程序运行

- **Windows**环境下使用管理员账号运行程序，非管理员账号运行需手动更改系统DNS配置，增加DNS：127.0.0.1作为主DNS或唯一DNS，以确保系统进行域名解析时能正常走代理服务StealthDNS。

  - 双击`stealth-dns.exe`文件或在命令提示符中通过指令`stealth-dns.exe run`来运行。
  - 通过`stealth-ui.exe`文件来运行客户端代理服务。

- **Linux与macOS**环境下使用`sudo`指令或root账号来运行程序，否则StealthDNS服务将无法监听53端口，同时不能增加DNS代理地址127.0.0.1。

  - 在终端中通过指令`sudo stealth-dns run`来运行。
  - 通过`stealth-ui`文件来运行客户端代理服务。
  
- 注意：代理服务StealthDNS如未将127.0.0.1设置成主DNS，请手动修改DNS配置将127.0.0.1设置为主DNS，确保域名解析请求能正常访问代理服务StealthDNS进行域名解析。





### 1.6 访问与证书

当StealthDNS服务成功启动后，通过浏览器在地址栏中输入`https://demo.nhp`进行访问。

域名`demo.nhp`的解析由StealthDNS完成，由于`demo.nhp`是一个自定义域名并不是真正的互联网域名，在对该域名访问时浏览器会因证书域名不匹配而阻止继续访问。

为保障浏览器的安全访问，需要进行如下操作步骤：

- 通过程序安装域名`demo.nhp`的认证根证书来保证对服务的正常访问。

  根证书导入或生成指令

  `stealth-dns install-root-ca`或`stealth-dns i`

  证书相关其他指令可以查看程序帮助文档`stealth-dns --help`

  

  