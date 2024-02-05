
### Go发展史
- 引入对WebAssembly的支持，让Gopher可以使用Go语言来开发Web应用；
- 提供了GOPRIVATE变量，用于指示哪些仓库下的module是私有的，即既不需要通过GOPROXY下载，也不需要通过GOSUMDB去验证其校验和。
- 在标准库中增加errors.Is和errors.As函数来解决错误值（error value）的比较判定问题，增加errors.Unwrap函数来解决error的展开（unwrap）问题。
- GODEBUG环境变量支持跟踪包init函数的消耗；
- 新增io/fs包，建立Go原生文件系统抽象；新增embed包，作为在二进制文件中嵌入静态资源文件的官方方案。

#### 开源社区对Go版本的选择策略
- 更近最新版本，像kubernetes
- 使用2个发布周期前的版本，像docker
- 使用最新版本之前的版本

