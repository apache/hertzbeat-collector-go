# Golang 代码 编程规范

Go 代码应该以最简单的方式编写，以实现其目标，无论是在行为还是性能方面。在 Google Go 代码库中，简单代码：

- 从上到下都易于阅读；
- 不假设你已经知道它在做什么；
- 不假设你可以记住所有前面的代码；
- 没有不必要的抽象层次；
- 没有能引起关注的世俗名称；
- 让开发者清楚地了解价值观和决策的传播；
- 有注释解释代码为什么这样做，而不是做什么，以避免将来出现偏差；
- 有独立的文档；
- 有 有用的错误和有用的失败测试；
- 可能经常与“聪明”的代码互相排斥。

参考指南：

- **https://google.github.io/styleguide/go/best-practices**
- **https://go.dev/wiki/CodeReviewComments**

- https://google.github.io/styleguide/go/
- https://go.dev/doc/effective_go

## 1. gofmt 统一代码格式

执行 `go fmt ./...`来统一代码格式。

## 2. 注释说明增强

在会引发歧义或者和上文判断不同时使用注释以提醒开发者。

```golang
if err := doSomething(); err != nil {
    // ...
}

if err := doSomething(); err == nil { // if NO error
    // ...
}
```

## 3. 注释应该说明的是”为什么“，而不是”做什么“

```go
// ❌ Bad
// Increment i by 1
i = i + 1

// ✅ Good
// Retry count is increased after each failed attempt.
i++
```

## 4. 当有 context.Context 参数时，其永远是方法的第一个参数并且不被放在结构体中

```golang
// ❌ Bad
type ServerContext struct {
  ctx context.Context
  // other 
}

func (s *ServerContext) doSomething(param any, ctx context.Context) {
  // ...
}

// ✅ Good
type ServerContext struct {
  // other 
}

func (s *ServerContext) doSomething(ctx context.Context, param any) {
  // ...
}
```

## 5. 正确声明变量

```golang
// ❌ Bad
// nil 
s := []int8{}

// ✅ Good
// 非零但长度为零
var s []int

i := 42
```

[]T{} 会默认分配内存，var s []T 更清晰与高效。

在使用非零值初始化时，使用  `:=` 赋值。

## 6. 代码中不要出现 panic

```golang
// ❌ Bad
func ParseConfig(file *os.File, cfg any) {
    decoder := yaml.NewDecoder(file)
    if err := decoder.Decode(cfg); err != nil {
        panic("failed to decode config: " + err.Error())
    }
}

// ✅ Good
func Parse(file *os.File, cfg any) error {
    decoder := yaml.NewDecoder(file)
    if err := decoder.Decode(cfg); err != nil {
        return err
    }
    return nil
}
```

依赖库应该优先返回 err，而不是终止程序。

## 7. 保持错误信息干净，使用结构化日志

```golang
// ❌ Bad
return fmt.Errorf("some error.")

// ✅ Good
return fmt.Errorf("some error.")
```

error msg 小写开头且不以任何标点结尾。

## 8. 避免多余的 else 逻辑

```golang
// ❌ Bad
if err != nil {
    if something {
        return err
    }
} else {
    doSomething()
}

// ✅ Good
if err != nil {
    return err
}
doSomething()
```

尽早返回 error 且避免多余的 else 语句。

## 9. 使用包别名导入

除非为了避免名称冲突，否则应避免重命名导入；好的包名称不应该需要重命名。如果发生冲突，优先重命名本地或项目特定的导入。

import pkg 按组组织，每组之间以空行分隔。标准库包始终位于第一组，其次是第三方包，最后是项目内部包。

## 10. 缩写保持大写和官方库一致

```golang
// ❌ Bad
type HttpServer struct{}

// ✅ Good
type HTTPServer struct{}
```

标准库写法：HTTP、ID、JSON。保持一致性.

## 11. 避免裸返回

```golang
// ❌ Bad
func sum(a, b int) (result int) {
    result = a + b
    return
}

// ✅ Good
func sum(a, b int) int {
    return a + b
}
```

除非函数非常短，一眼看到底时使用裸返回。

## 12. 包名、接受者名保持简单与简洁切避免函数名重复与啰嗦

```golang
// ❌ Bad
package my_utils

func (this *Server) Start() {}

package yamlconfig
func ParseYAMLConfig(input string) (*Config, error)

func OverrideFirstWithSecond(dest, source *Config) error

// 不应书写函数的返回参数名,这会导致 goDoc 混乱
func doSomething(a, b int) (sum int, err error) {
  // logic
}

// ✅ Good
package utils

func (s *Server) Start() 

package yamlconfig
func Parse(input string) (*Config, error)

func Override(dest, source *Config) error

func doSomething(a, b int) (int,error) {
  // logic
}
```

- 包名小写、简洁、无下划线；
- 接受者变量 `s`, `r`, `c` 这种短变量常见且清晰；
- 包名提供上下文。

当需要消除类似名称的函数歧义时，可以包含额外的信息。

```golang
// ✅ Good
func (c *Config) WriteTextTo(w io.Writer) (int64, error)
func (c *Config) WriteBinaryTo(w io.Writer) (int64, error)
```

## 13. 函数名中不应出现返回类型且命名体现语义

```golang
// ❌ Bad
func TransformToJSON(input *Config) *jsonconfig.Config

func (c *Config) GetJobName(key string) (value string, ok bool)

// ✅ Good
func Transform(input *Config) *jsonconfig.Config

// 返回值函数中不要动词
func (c *Config) JobName(key string) (value string, ok bool)

// 动作用动词，取值用名词
func (c *Config) ApplyChanges() error
func ProcessData(data []byte) error
```

动作用动词，取值用名词且返回值函数中不要动词。

## 14. 测试写法符合 Go `got %v, want %v`风格

```golang
// ❌ Bad
if got != want {
    t.Errorf("expected %v but got %v", want, got)
}

// ✅ Good
if got != want {
    t.Errorf("got %v, want %v", got, want)
}
```

## 15. 常量与结构化错误声明

```golang
type Animal string

var (
    // ErrDuplicate occurs if this animal has already been seen.
    ErrDuplicate = errors.New("duplicate")

    // ErrMarsupial occurs because we're allergic to marsupials outside Australia.
    // Sorry.
    ErrMarsupial = errors.New("marsupials are not supported")
)

func process(animal Animal) error {
    switch {
    case seen[animal]:
        return ErrDuplicate
    case marsupial(animal):
        return ErrMarsupial
    }
    seen[animal] = true
    // ...
    return nil
}
```

在使用到常量定义或者错误时，将其机构化定义在文件顶部或统一管理。

## 16. 当函数参数列表过多时，使用可变参数处理输入

```golang
type SshProtocol struct {
 Host                 string
 Port                 string
 Timeout              string
 Username             string
 Password             string
 PrivateKey           string
 PrivateKeyPassphrase string
 ReuseConnection      string
 Script               string
 ParseType            string
 ProxyHost            string
 ProxyPort            string
 ProxyUsername        string
 ProxyPassword        string
 UseProxy             string
 ProxyPrivateKey      string
}

type SshProtocolConfigOptFunc func(option *SshProtocol)

func NewSshProtocol(host, port string, opts ...SshProtocolConfigOptFunc) *SshProtocol {

 option := &SshProtocol{
  Host: host,
  Port: port,
 }

 for _, opt := range opts {
  opt(option)
 }

 return &SshProtocol{
  Host:                 host,
  Port:                 port,
  Timeout:              option.Timeout,
  Username:             option.Username,
  Password:             option.Password,
  PrivateKey:           option.PrivateKey,
  PrivateKeyPassphrase: option.PrivateKeyPassphrase,
  ReuseConnection:      option.ReuseConnection,
  Script:               option.Script,
  ParseType:            option.ParseType,
  ProxyHost:            option.ProxyHost,
  ProxyPort:            option.ProxyPort,
  ProxyUsername:        option.ProxyUsername,
  ProxyPassword:        option.ProxyPassword,
  UseProxy:             option.UseProxy,
  ProxyPrivateKey:      option.ProxyPrivateKey,
 }
}

func (sp *SshProtocol) IsInvalid() error {

 return nil
}
```

## 17. 字符串拼接

```golang
// 连接少量字符时使用 + 
key := "projectid: " + p

// 构建带有格式化的复杂字符串时，优先使用 fmt.Sprintf
str := fmt.Sprintf("%s [%s:%d]-> %s", src, qos, mtu, dst)

// 字符串格式更加复杂时，优先使用 text/template 或 safehtml/template
```
