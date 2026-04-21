## 1 Compilation and Execution

### 1.1 Environment Setup

#### 1.1.1 Go Environment Setup

Go Language environment: **Go 1.26.2** . Installation package download: <https://go.dev/dl/>
- **Windows and macOS** Environment, install Go through the downloaded installer.
- **Linux** environment can be installed directly through the management tool: `sudo apt install golang`.Alternatively, perform a manual installation using the following commands:

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

- After the installation is successful, run the command `go version`to see the Go version number.

#### 1.1.2 Wails Environment Preparation

Wails has several common dependencies required before installation:

- Go 1.21+ (macOS 15+ requires Go 1.23.3+)
- NPM (Node 15+)

Wails Platform-Specific Dependencies:

- **Windows**: Wails requires the [WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) runtime to be installed.
- **MacOS**: Wails requires the Xcode command-line tools to be installed. This can be done by running `xcode-select --install`.
- **Linux**: Linux requires standard `gcc` build tools, along with `libgtk3` and `libwebkit`.

Download NPM from the [Node download page](https://nodejs.org/en/download/), and run `npm --version` to verify the installation.

Run `go install github.com/wailsapp/wails/v2/cmd/wails@latest` to install the Wails CLI.

After successfully installing Wails, run `wails doctor`. This will check if you have the correct dependencies installed. If not, it will provide suggestions on what is missing to help you correct the issues.

<small>*(If your system reports that the `wails` command is missing, ensure you have correctly followed the Go installation guide. Usually, this means the `go/bin` directory in your user's home directory is not in the `PATH` environment variable. It's also often necessary to close and reopen any open command prompts so that changes made by the installer to the environment are reflected in the command prompt.)*</small>

If you have any questions about the Wails installation, please refer to the [Wails official installation instructions](https://wails.io/docs/gettingstarted/installation/) for installing Wails.

### 1.2 Compilation

- **Windows**: Compile the project by executing the batch file `build.bat` located in the project root directory.
  - `build.bat`: Compiles only the stealth-dns.exe executable, which can run independently to perform DNS proxying.
  - `build.bat ui`: Compiles only the stealth-ui.exe executable. This UI application requires stealth-dns.exe to perform DNS proxying.
  - `build.bat full`: Compiles both stealth-dns.exe and stealth-ui.exe executables simultaneously.
- **Linux and macOS**: Compile the project by running the `make` command in the project root directory.
  - `build.bat`: Compiles only the stealth-dns executable, which can run independently to perform DNS proxying.
  - `build.bat ui`: Compiles only the stealth-ui executable. This UI application requires stealth-dns to perform DNS proxying.
  - `build.bat full`: Compiles both stealth-dns and stealth-ui executables simultaneously.
- The compiled executable files are generated in the `release/` subdirectory.
- Configuration files are placed in the `release/etc/` subdirectory. These files are copied from the `etc/` subdirectory during the build process.

### 1.3 Configuration Files

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



### 1.5 Execution

- **Windows**: Run the program with Administrator privileges. If running without Administrator rights, manually change the system DNS configuration: add `127.0.0.1` as the primary or sole DNS server to ensure domain name resolution is routed through the StealthDNS proxy service.

  - Double-click the `stealth-dns.exe` file or run it via the command `stealth-dns.exe run` in the command prompt.
  - Run the client proxy service via the `stealth-ui.exe` file.

- **Linux and macOS**: Run the program using the `sudo` command or as the `root` user. Otherwise, the StealthDNS service will be unable to listen on port 53, and the DNS proxy address `127.0.0.1` cannot be added.
  - Run it in the terminal using the command `sudo stealth-dns run`.
  - Run the client proxy service via the `stealth-ui` file.
  
- **Note:** If the StealthDNS proxy service has not set 127.0.0.1 as the primary DNS, please manually modify the DNS configuration to set 127.0.0.1 as the primary DNS. This ensures that domain name resolution requests can properly reach the StealthDNS proxy service for domain resolution.



### 1.6 Access and Certificates

Once the StealthDNS service starts successfully, access it by entering `https://demo.nhp` in the address bar of your browser.

The domain name `demo.nhp` is resolved by StealthDNS. Since `demo.nhp` is a custom domain name and not an actual internet domain, the browser will block access due to a certificate domain name mismatch when attempting to visit this domain.

To ensure secure browser access, follow these steps:

- Install the root certificate for the domain `demo.nhp` using the program to enable normal access to the service.

  Root certificate import or generation command:

  `stealth-dns install-root-ca` or `stealth-dns i`

  For other certificate-related commands, refer to the program help documentation: `stealth-dns --help`
