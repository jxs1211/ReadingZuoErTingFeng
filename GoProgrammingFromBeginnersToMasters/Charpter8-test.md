# 第八部分 测试、性能剖析与调试

Go语言推崇“面向工程”的设计哲学并自带强大且为人所称道的工具链，本部分将详细介绍Go在单元测试、性能测试以及代码调试方面的最佳实践方案。

## 第40条 理解包内测试与包外测试的差别

Go语言在工具链和标准库中提供对测试的原生支持，这算是Go语言在工程实践方面的一个创新，也是Go相较于其他主流语言的一个突出亮点。

在Go中我们针对包编写测试代码。测试代码与包代码放在同一个包目录下，并且Go要求所有测试代码都存放在以*_test.go结尾的文件中。这使Go开发人员一眼就能分辨出哪些文件存放的是包代码，哪些文件存放的是针对该包的测试代码。

go test命令也是通过同样的方式将包代码和包测试代码区分开的。go test将所有包目录下的*_test.go文件编译成一个临时二进制文件（可以通过go test -c显式编译出该文件），并执行该文件，后者将执行各个测试源文件中名字格式为TestXxx的函数所代表的测试用例并输出测试执行结果。


### 40.1　官方文档的“自相矛盾”
Go原生支持测试的两大要素——go test命令和testing包，它们是Gopher学习Go代码测试的必经之路。
下面是关于testing包的一段官方文档（Go 1.14版本）摘录：
>要编写一个新的测试集（test suite），创建一个包含TestXxx函数的以_test.go为文件名结尾的文件。将这个测试文件放在与被测试包相同的包下面。编译被测试包时，该文件将被排除在外；执行go test时，该文件将被包含在内。

同样是官方文档，在介绍go test命令行工具时，Go文档则如是说：
>那些包名中带有_test后缀的测试文件将被编译成一个独立的包，这个包之后会被链接到主测试二进制文件中并运行。

对比这两段官方文档，我们发现了一处“自相矛盾”[1]的地方：testing包文档告诉我们将测试代码放入与被测试包同名的包中，而go test命令行帮助文档则提到会将包名中带有_test后缀的测试文件编译成一个独立的包。

我们用一个例子来直观说明一下这个“矛盾”：如果我们要测试的包为foo，testing包的帮助文档告诉我们把对foo包的测试代码放在包名为foo的测试文件中；而go test命令行帮助文档则告诉我们把foo包的测试代码放在包名为foo_test的测试文件中。

我们把将测试代码放在与被测包同名的包中的测试方法称为“包内测试”。可以通过下面的命令查看哪些测试源文件使用了包内测试：
```sh
$go list -f={{.TestGoFiles}} .
```
我们把将测试代码放在名为被测包包名+"_test"的包中的测试方法称为“包外测试”。可以通过下面的命令查看哪些测试源文件使用了包外测试：
```sh
$go list -f={{.XTestGoFiles}} .
```
那么我们究竟是选择包内测试还是包外测试呢？在给出结论之前，我们将分别对这两种方法做一个详细分析。
>https://github.com/golang/go/issues/2522

### 40.2　包内测试与包外测试

#### 1. Go标准库中包内测试和包外测试的使用情况

在$GOROOT/src目录下（Go 1.14版本），执行下面的命令组合：
```sh
// 统计标准库中采用包内测试的测试文件数量
$find . -name "*_test.go" |xargs grep package |grep ':package'|grep -v "_test$"|wc -l
     691

// 统计标准库中采用包外测试的测试文件数量
$find . -name "*_test.go" |xargs grep package |grep ':package'|grep "_test$"  |wc -l
     448
```
并非精确的统计，但能在一定程度上说明包内测试和包外测试似乎各有优势。我们再以net/http这个被广泛使用的明星级的包为例，看包内测试和包外测试在该包测试中的应用。

进入$GOROOT/src/net/http目录下，分别执行下面命令：
```sh
$go list -f={{.XTestGoFiles}}
[alpn_test.go client_test.go clientserver_test.go example_filesystem_test.go example_handle_test.go example_test.go fs_test.go main_test.go
    request_test.go serve_test.go sniff_test.go transport_test.go]
$go list -f={{.TestGoFiles}}
[cookie_test.go export_test.go filetransport_test.go header_test.go
    http_test.go proxy_test.go range_test.go readrequest_test.go
    requestwrite_test.go response_test.go responsewrite_test.go
    server_test.go transfer_test.go transport_internal_test.go]
```
我们看到，在针对net/http的测试代码中，对包内测试和包外测试的使用仍然不分伯仲。

#### 2. 包内测试的优势与不足
由于Go构建工具链在编译包时会自动根据文件名是否具有_test.go后缀将包源文件和包的测试源文件分开，测试代码不会进入包正常构建的范畴，因此测试代码使用与被测包名相同的包内测试方法是一个很自然的选择。

包内测试这种方法本质上是一种白盒测试方法。由于测试代码与被测包源码在同一包名下，测试代码可以访问该包下的所有符号，无论是导出符号还是未导出符号；并且由于包的内部实现逻辑对测试代码是透明的，包内测试可以更为直接地构造测试数据和实施测试逻辑，可以很容易地达到较高的测试覆盖率。因此对于追求高测试覆盖率的项目而言，包内测试是不二之选。

在实践中，实施包内测试也经常会遇到如下问题。

1）测试代码自身需要经常性的维护

包内测试的白盒测试本质意味着它是一种面向实现的测试。测试代码的测试数据构造和测试逻辑通常与被测包的特定数据结构设计和函数/方法的具体实现逻辑是紧耦合的。这样一旦被测包的数据结构设计出现调整或函数/方法的实现逻辑出现变动，那么对应的测试代码也要随之同步调整，否则整个包将无法通过测试甚至测试代码本身的构建都会失败。而包的内部实现逻辑又是易变的，其优化调整是一种经常性行为，这就意味着采用包内测试的测试代码也需要经常性的维护。

2）硬伤：包循环引用

采用包内测试可能会遇到一个绕不过去的硬伤：包循环引用。我们看图40-1。

从图40-1中我们看到，对包c进行测试的代码（c_test.go）采用了包内测试的方法，其测试代码位于包c下面，测试代码导入并引用了包d，而包d本身却导入并引用了包c，这种包循环引用是Go编译器所不允许的。

如果Go标准库对strings包的测试采用包内测试会遭遇什么呢？见图40-2。

<!-- testing.go -->
testing package import "strings"
<!-- strings_test.go -->
strings package import "testing"

对标准库strings进行包内测试将遭遇“包循环引用”

从图40-2中我们看到，Go测试代码必须导入并引用的testing包引用了strings包，这样如果strings包仍然使用包内测试方法，就必然会在测试代码中出现strings包与testing包循环引用的情况。于是当我们在标准库strings包目录下执行下面命令时，我们得到：
```sh
// 在$GOROOT/src/strings目录下
$go list -f {{.TestGoFiles}} .
[export_test.go]
```
我们看到标准库strings包并未采用包内测试的方法（注：export_test.go并非包内测试的测试源文件，这一点后续会有详细说明）。

#### 3. 包外测试（仅针对导出API的测试）

因为“包循环引用”的事实存在，Go标准库无法针对strings包实施包内测试，而解决这一问题的自然就是包外测试了：
```sh
// 在$GOROOT/src/strings目录下
$go list -f {{.XTestGoFiles}} .
[builder_test.go compare_test.go example_test.go reader_test.go replace_test.go search_test.go strings_test.go]
```
与包内测试本质是面向实现的白盒测试不同，包外测试的本质是一种面向接口的黑盒测试。这里的“接口”指的就是被测试包对外导出的API，这些API是被测包与外部交互的契约。契约一旦确定就会长期保持稳定，无论被测包的内部实现逻辑和数据结构设计如何调整与优化，一般都不会影响这些契约。这一本质让包外测试代码与被测试包充分解耦，使得针对这些导出API进行测试的包外测试代码表现出十分健壮的特性，即很少随着被测代码内部实现逻辑的调整而进行调整和维护。

包外测试将测试代码放入不同于被测试包的独立包的同时，也使得包外测试不再像包内测试那样存在“包循环引用”的硬伤。还以标准库中的strings包为例，见图40-3。


```sh
// string_test.go
package strings_test

import (
  ...
	. "strings"
	"testing"
  ...
)
```

库strings包采用包外测试后解决了“包循环引用”问题

从图40-3中我们看到，采用包外测试的strings包将测试代码放入strings_test包下面，strings_test包既引用了被测试包strings，又引用了testing包，这样一来原先采用包内测试的strings包与testing包的循环引用被轻易地“解”开了。

包外测试这种纯黑盒的测试还有一个功能域之外的好处，那就是可以更加聚焦地从用户视角验证被测试包导出API的设计的合理性和易用性。

不过包外测试的不足也是显而易见的，那就是存在测试盲区。由于测试代码与被测试目标并不在同一包名下，测试代码仅有权访问被测包的导出符号，并且仅能通过导出API这一有限的“窗口”并结合构造特定数据来验证被测包行为。在这样的约束下，很容易出现对被测试包的测试覆盖不足的情况。

Go标准库的实现者们提供了一个解决这个问题的惯用法：安插后门。这个后门就是前面曾提到过的export_test.go文件。该文件中的代码位于被测包名下，但它既不会被包含在正式产品代码中（因为位于_test.go文件中），又不包含任何测试代码，而仅用于将被测包的内部符号在测试阶段暴露给包外测试代码：
```go
// $GOROOT/src/fmt/export_test.go
package fmt

var IsSpace = isSpace
var Parsenum = parsenum
```
或者定义一些辅助包外测试的代码，比如扩展被测包的方法集合：
```go
// $GOROOT/src/strings/export_test.go
package strings

func (r *Replacer) Replacer() interface{} {
    r.once.Do(r.buildOnce)
    return r.r
}

func (r *Replacer) PrintTrie() string {
    r.once.Do(r.buildOnce)
    gen := r.r.(*genericReplacer)
    return gen.printNode(&gen.root, 0)
}
...
```
我们可以用图40-4来直观展示export_test.go这个后门在不同阶段的角色（以fmt包为例）。

从图40-4中可以看到，export_test.go仅在go test阶段与被测试包（fmt）一并被构建入最终的测试二进制文件中。在这个过程中，包外测试代码（fmt_test）可以通过导入被测试包（fmt）来访问export_test.go中的导出符号（如IsSpace或对fmt包的扩展）。而export_test.go相当于在测试阶段扩展了包外测试代码的视野，让很多本来很难覆盖到的测试路径变得容易了，进而让包外测试覆盖更多被测试包中的执行路径。

#### 4. 优先使用包外测试

经过上面的比较，我们发现包内测试与包外测试各有优劣，那么在Go测试编码实践中我们究竟该选择哪种测试方式呢？关于这个问题，目前并无标准答案。基于在实践中开发人员对编写测试代码的热情和投入时间，笔者更倾向于优先选择包外测试，理由如下。包外测试可以：

```
优先保证被测试包导出API的正确性；
可从用户角度验证导出API的有效性；
保持测试代码的健壮性，尽可能地降低对测试代码维护的投入；
不失灵活！可通过export_test.go这个“后门”来导出我们需要的内部符号，满足窥探包内实现逻辑的需求。
```
当然go test也完全支持对被测包同时运用包内测试和包外测试两种测试方法，就像标准库net/http包那样。在这种情况下，包外测试由于将测试代码放入独立的包中，它更适合编写偏向集成测试的用例，它可以任意导入外部包，并测试与外部多个组件的交互。比如：net/http包的serve_test.go中就利用httptest包构建的模拟Server来测试相关接口。而包内测试更聚焦于内部逻辑的测试，通过给函数/方法传入一些特意构造的数据的方式来验证内部逻辑的正确性，比如

net/http包的response_test.go。我们还可以通过测试代码的文件名来区分所属测试类别，比如：net/http包就使用transport_internal_test.go这个名字来明确该测试文件采用包内测试的方法，而对应的transport_test.go则是一个采用包外测试的源文件。

小结

在这一条中，我们了解了go test的执行原理，对比了包内测试和包外测试各自的优点和不足，并给出了在实际开发过程中选择测试类型的建议。

本条要点：
```
go test执行测试的原理；
理解包内测试的优点与不足；
理解包外测试的优点与不足；
掌握通过export_test.go为包外测试添加“后门”的惯用法；
优先使用包外测试；
```
当运用包外测试与包内测试共存的方式时，可考虑让包外测试和包内测试聚焦于不同的测试类别。


## 第41条有层次地组织测试代码

上一条明确了测试代码放置的位置（包内测试或包外测试）。在这一条中，我们来聚焦位于测试包内的测试代码该如何组织。

### 41.1　经典模式——平铺
Go从对外发布的那一天起就包含了go test命令，这个命令会执行_test.go中符合TestXxx命名规则的函数进而实现测试代码的执行。go test并没有对测试代码的组织提出任何约束条件。于是早期的测试代码采用了十分简单直接的组织方式——平铺。

下面是对Go 1.5版本标准库strings包执行测试后的结果：
```sh
# go test -v .
=== RUN   TestCompare
--- PASS: TestCompare (0.00s)
=== RUN   TestCompareIdenticalString
--- PASS: TestCompareIdenticalString (0.00s)
=== RUN   TestCompareStrings
--- PASS: TestCompareStrings (0.00s)
=== RUN   TestReader
--- PASS: TestReader (0.00s)
...
=== RUN   TestEqualFold
--- PASS: TestEqualFold (0.00s)
=== RUN   TestCount
--- PASS: TestCount (0.00s)
...
PASS
ok    strings     0.457s
```
我们看到，以strings包的Compare函数为例，与之对应的测试函数有三个：TestCompare、TestCompareIdenticalString和TestCompareStrings。这些测试函数各自独立，测试函数之间没有层级关系，所有测试平铺在顶层。测试函数名称既用来区分测试，又用来关联测试。我们通过测试函数名的前缀才会知道，TestCompare、TestCompareIdenticalString和TestCompareStrings三个函数是针对strings包Compare函数的测试。

在go test命令中，我们还可以通过给命令行选项-run提供正则表达式来匹配并选择执行哪些测试函数。还以strings包为例，下面的命令仅执行测试函数名字中包含TestCompare前缀的测试：

```sh
# go test -run=TestCompare -v .
=== RUN   TestCompare
--- PASS: TestCompare (0.00s)
=== RUN   TestCompareIdenticalString
--- PASS: TestCompareIdenticalString (0.00s)
=== RUN   TestCompareStrings
--- PASS: TestCompareStrings (0.00s)
PASS
ok    strings     0.088s
```
平铺模式的测试代码组织方式的优点是显而易见的。
```
简单：没有额外的抽象，上手容易。
独立：每个测试函数都是独立的，互不关联，避免相互干扰。
```
### 41.2　xUnit家族模式
在Java、Python、C#等主流编程语言中，测试代码的组织形式深受由极限编程倡导者Kent Beck和Erich Gamma建立的xUnit家族测试框架（如JUnit、PyUnit等）的影响。

使用了xUnit家族单元测试框架的典型测试代码组织形式（这里称为xUnit家族模式）如图41-1所示。

Unit家族单元测试代码组织形式

我们看到这种测试代码组织形式主要有测试套件（Test Suite）和测试用例（Test Case）两个层级。一个测试工程（Test Project）由若干个测试套件组成，而每个测试套件又包含多个测试用例。

在Go 1.7版本之前，使用Go原生工具和标准库是无法按照上述形式组织测试代码的。但Go 1.7中加入的对subtest的支持让我们在Go中也可以使用上面这种方式组织Go测试代码。还以上面标准库strings包的测试代码为例，这里将其部分测试代码的组织形式改造一下（代码较多，这里仅摘录能体现代码组织形式的必要代码）：
```go
// chapter8/sources/strings-test-demo/compare_test.go
package strings_test

...

func testCompare(t *testing.T) {
    ...
}

func testCompareIdenticalString(t *testing.T) {
    ...
}

func testCompareStrings(t *testing.T) {
    ...
}

func TestCompare(t *testing.T) {
    t.Run("Compare", testCompare)
    t.Run("CompareString", testCompareStrings)
    t.Run("CompareIdenticalString", testCompareIdenticalString)
}

// chapter8/sources/strings-test-demo/builder_test.go
package strings_test

...

func testBuilder(t *testing.T) {
    ...
}
func testBuilderString(t *testing.T) {
    ...
}
func testBuilderReset(t *testing.T) {
    ...
}
func testBuilderGrow(t *testing.T) {
    ...
}

func TestBuilder(t *testing.T) {
    t.Run("TestBuilder", testBuilder)
    t.Run("TestBuilderString", testBuilderString)
    t.Run("TestBuilderReset", testBuilderReset)
    t.Run("TestBuilderGrow", testBuilderGrow)
}
```

造前后测试代码的组织结构对比如图41-2所示。

从图41-2中我们看到，改造后的名字形如TestXxx的测试函数对应着测试套件，一般针对被测包的一个导出函数或方法的所有测试都放入一个测试套件中。平铺模式下的测试函数TestXxx都改名为testXxx，并作为测试套件对应的测试函数内部的子测试（subtest）。上面的代码中，原先的TestBuilderString变为了testBuilderString。这样的一个子测试等价于一个测试用例。通过对比，我们看到，仅通过查看测试套件内的子测试（测试用例）即可全面了解到究竟对被测函数/方法进行了哪些测试。仅仅增加了一个层次，测试代码的组织就更加清晰了。

运行一下改造后的测试：

```sh
$go test -v .
=== RUN   TestBuilder
=== RUN   TestBuilder/TestBuilder
=== RUN   TestBuilder/TestBuilderString
=== RUN   TestBuilder/TestBuilderReset
=== RUN   TestBuilder/TestBuilderGrow
--- PASS: TestBuilder (0.00s)
    --- PASS: TestBuilder/TestBuilder (0.00s)
    --- PASS: TestBuilder/TestBuilderString (0.00s)
    --- PASS: TestBuilder/TestBuilderReset (0.00s)
    --- PASS: TestBuilder/TestBuilderGrow (0.00s)
=== RUN   TestCompare
=== RUN   TestCompare/Compare
=== RUN   TestCompare/CompareString
=== RUN   TestCompare/CompareIdenticalString
--- PASS: TestCompare (0.44s)
    --- PASS: TestCompare/Compare (0.00s)
    --- PASS: TestCompare/CompareString (0.44s)
    --- PASS: TestCompare/CompareIdenticalString (0.00s)
PASS
ok         strings-test-demo     0.446s
```
### 41.3　测试固件
无论测试代码是采用传统的平铺模式，还是采用基于测试套件和测试用例的xUnit实践模式进行组织，都有着对测试固件（test fixture）的需求。

测试固件是指一个人造的、确定性的环境，一个测试用例或一个测试套件（下的一组测试用例）在这个环境中进行测试，其测试结果是可重复的（多次测试运行的结果是相同的）。我们一般使用setUp和tearDown来代表测试固件的创建/设置与拆除/销毁的动作。

下面是一些使用测试固件的常见场景：

```
将一组已知的特定数据加载到数据库中，测试结束后清除这些数据；
复制一组特定的已知文件，测试结束后清除这些文件；
创建伪对象（fake object）或模拟对象（mock object），并为这些对象设定测试时所需的特定数据和期望结果。
```

在传统的平铺模式下，由于每个测试函数都是相互独立的，因此一旦有对测试固件的需求，我们需要为每个TestXxx测试函数单独创建和销毁测试固件。看下面的示例：

```go
// chapter8/sources/classic_testfixture_test.go
package demo_test
...
func setUp(testName string) func() {
    fmt.Printf("\tsetUp fixture for %s\n", testName)
    return func() {
        fmt.Printf("\ttearDown fixture for %s\n", testName)
    }
}

func TestFunc1(t *testing.T) {
    defer setUp(t.Name())()
    fmt.Printf("\tExecute test: %s\n", t.Name())
}

func TestFunc2(t *testing.T) {
    defer setUp(t.Name())()
    fmt.Printf("\tExecute test: %s\n", t.Name())
}

func TestFunc3(t *testing.T) {
    defer setUp(t.Name())()
    fmt.Printf("\tExecute test: %s\n", t.Name())
}
```

上面的示例在运行每个测试函数TestXxx时，都会先通过setUp函数建立测试固件，并在defer函数中注册测试固件的销毁函数，以保证在每个TestXxx执行完毕时为之建立的测试固件会被销毁，使得各个测试函数之间的测试执行互不干扰。

在Go 1.14版本以前，测试固件的setUp与tearDown一般是这么实现的：
```go
func setUp() func(){
    ...
    return func() {
    }
}

func TestXxx(t *testing.T) {
    defer setUp()()
    ...
}
```
在setUp中返回匿名函数来实现tearDown的好处是，可以在setUp中利用闭包特性在两个函数间共享一些变量，避免了包级变量的使用。

Go 1.14版本testing包增加了testing.Cleanup方法，为测试固件的销毁提供了包级原生的支持：
```go
func setUp() func(){
    ...
    return func() {
    }
}

func TestXxx(t *testing.T) {
    t.Cleanup(setUp())
    ...
}
```
有些时候，我们需要将所有测试函数放入一个更大范围的测试固件环境中执行，这就是包级别测试固件。在Go 1.4版本以前，我们仅能在init函数中创建测试固件，而无法销毁包级别测试固件。Go 1.4版本引入了TestMain，使得包级别测试固件的创建和销毁终于有了正式的施展舞台。看下面的示例：

```go
// chapter8/sources/classic_package_level_testfixture_test.go
package demo_test

...
func setUp(testName string) func() {
    fmt.Printf("\tsetUp fixture for %s\n", testName)
    return func() {
        fmt.Printf("\ttearDown fixture for %s\n", testName)
    }
}

func TestFunc1(t *testing.T) {
    t.Cleanup(setUp(t.Name()))
    fmt.Printf("\tExecute test: %s\n", t.Name())
}

func TestFunc2(t *testing.T) {
    t.Cleanup(setUp(t.Name()))
    fmt.Printf("\tExecute test: %s\n", t.Name())
}

func TestFunc3(t *testing.T) {
    t.Cleanup(setUp(t.Name()))
    fmt.Printf("\tExecute test: %s\n", t.Name())
}

func pkgSetUp(pkgName string) func() {
    fmt.Printf("package SetUp fixture for %s\n", pkgName)
    return func() {
        fmt.Printf("package TearDown fixture for %s\n", pkgName)
    }
}

func TestMain(m *testing.M) {
    defer pkgSetUp("package demo_test")()
    m.Run()
}
```
运行该示例：

```sh
$go test -v classic_package_level_testfixture_test.go
package SetUp fixture for package demo_test
=== RUN   TestFunc1
    setUp fixture for TestFunc1
    Execute test: TestFunc1
    tearDown fixture for TestFunc1
--- PASS: TestFunc1 (0.00s)
=== RUN   TestFunc2
    setUp fixture for TestFunc2
    Execute test: TestFunc2
    tearDown fixture for TestFunc2
--- PASS: TestFunc2 (0.00s)
=== RUN   TestFunc3
    setUp fixture for TestFunc3
    Execute test: TestFunc3
    tearDown fixture for TestFunc3
--- PASS: TestFunc3 (0.00s)
PASS
package TearDown fixture for package demo_test
ok    command-line-arguments   0.008s
```
有些时候，一些测试函数所需的测试固件是相同的，在平铺模式下为每个测试函数都单独创建/销毁一次测试固件就显得有些重复和冗余。在这样的情况下，我们可以尝试采用测试套件来减少测试固件的重复创建。来看下面的示例：
```go
// chapter8/sources/xunit_suite_level_testfixture_test.go
package demo_test

...
func suiteSetUp(suiteName string) func() {
    fmt.Printf("\tsetUp fixture for suite %s\n", suiteName)
    return func() {
        fmt.Printf("\ttearDown fixture for suite %s\n", suiteName)
    }
}

func func1TestCase1(t *testing.T) {
    fmt.Printf("\t\tExecute test: %s\n", t.Name())
}

func func1TestCase2(t *testing.T) {
    fmt.Printf("\t\tExecute test: %s\n", t.Name())
}

func func1TestCase3(t *testing.T) {
    fmt.Printf("\t\tExecute test: %s\n", t.Name())
}

func TestFunc1(t *testing.T) {
    t.Cleanup(suiteSetUp(t.Name()))
    t.Run("testcase1", func1TestCase1)
    t.Run("testcase2", func1TestCase2)
    t.Run("testcase3", func1TestCase3)
}

func func2TestCase1(t *testing.T) {
    fmt.Printf("\t\tExecute test: %s\n", t.Name())
}

func func2TestCase2(t *testing.T) {
    fmt.Printf("\t\tExecute test: %s\n", t.Name())
}

func func2TestCase3(t *testing.T) {
    fmt.Printf("\t\tExecute test: %s\n", t.Name())
}

func TestFunc2(t *testing.T) {
    t.Cleanup(suiteSetUp(t.Name()))
    t.Run("testcase1", func2TestCase1)
    t.Run("testcase2", func2TestCase2)
    t.Run("testcase3", func2TestCase3)
}

func pkgSetUp(pkgName string) func() {
    fmt.Printf("package SetUp fixture for %s\n", pkgName)
    return func() {
        fmt.Printf("package TearDown fixture for %s\n", pkgName)
    }
}

func TestMain(m *testing.M) {
    defer pkgSetUp("package demo_test")()
    m.Run()
}
```
这个示例采用了xUnit实践的测试代码组织方式，将对测试固件需求相同的一组测试用例放在一个测试套件中，这样就可以针对测试套件创建和销毁测试固件了。

运行一下该示例：
```sh
$go test -v xunit_suite_level_testfixture_test.go
package SetUp fixture for package demo_test
=== RUN   TestFunc1
   setUp fixture for suite TestFunc1
=== RUN   TestFunc1/testcase1
           Execute test: TestFunc1/testcase1
=== RUN   TestFunc1/testcase2
           Execute test: TestFunc1/testcase2
=== RUN   TestFunc1/testcase3
           Execute test: TestFunc1/testcase3
   tearDown fixture for suite TestFunc1
--- PASS: TestFunc1 (0.00s)
    --- PASS: TestFunc1/testcase1 (0.00s)
    --- PASS: TestFunc1/testcase2 (0.00s)
    --- PASS: TestFunc1/testcase3 (0.00s)
=== RUN   TestFunc2
   setUp fixture for suite TestFunc2
=== RUN   TestFunc2/testcase1
           Execute test: TestFunc2/testcase1
=== RUN   TestFunc2/testcase2
           Execute test: TestFunc2/testcase2
=== RUN   TestFunc2/testcase3
           Execute test: TestFunc2/testcase3
   tearDown fixture for suite TestFunc2
--- PASS: TestFunc2 (0.00s)
    --- PASS: TestFunc2/testcase1 (0.00s)
    --- PASS: TestFunc2/testcase2 (0.00s)
    --- PASS: TestFunc2/testcase3 (0.00s)
PASS
package TearDown fixture for package demo_test
ok    command-line-arguments   0.005s
```

当然在这样的测试代码组织方式下，我们仍然可以单独为每个测试用例创建和销毁测试固件，从而形成一种多层次的、更灵活的测试固件设置体系。可以用图41-4总结一下这种模式下的测试执行流。

小结

在确定了将测试代码放入包内测试还是包外测试之后，我们在编写测试前，还要做好测试包内部测试代码的组织规划，建立起适合自己项目规模的测试代码层次体系。简单的测试可采用平铺模式，复杂的测试可借鉴xUnit的最佳实践，利用subtest建立包、测试套件、测试用例三级的测试代码组织形式，并利用TestMain和testing.Cleanup方法为各层次的测试代码建立测试固件。

## 第42条 优先编写表驱动的测试

在前两条中，我们明确了测试代码放置的位置（包内测试或包外测试）以及如何根据实际情况更有层次地组织测试代码。在这一条中，我们将聚焦于测试函数的内部代码该如何编写。

### 42.1　Go测试代码的一般逻辑

众所周知，Go的测试函数就是一个普通的Go函数，Go仅对测试函数的函数名和函数原型有特定要求，对在测试函数TestXxx或其子测试函数（subtest）中如何编写测试逻辑并没有显式的约束。对测试失败与否的判断在于测试代码逻辑是否进入了包含Error/Errorf、Fatal/Fatalf等方法调用的代码分支。一旦进入这些分支，即代表该测试失败。不同的是Error/Errorf并不会立刻终止当前goroutine的执行，还会继续执行该goroutine后续的测试，而Fatal/Fatalf则会立刻停止当前goroutine的测试执行。

下面的测试代码示例改编自$GOROOT/src/strings/compare_test.go：
```go
// chapter8/sources/non_table_driven_strings_test.go
func TestCompare(t *testing.T) {
    var a, b string
    var i int

    a, b = "", ""
    i = 0
    cmp := strings.Compare(a, b)
    if cmp != i {
        t.Errorf(`want %v, but Compare(%q, %q) = %v`, i, a, b, cmp)
    }

    a, b = "a", ""
    i = 1
    cmp = strings.Compare(a, b)
    if cmp != i {
        t.Errorf(`want %v, but Compare(%q, %q) = %v`, i, a, b, cmp)
    }

    a, b = "", "a"
    i = -1
    cmp = strings.Compare(a, b)
    if cmp != i {
        t.Errorf(`want %v, but Compare(%q, %q) = %v`, i, a, b, cmp)
    }
}
```
上述示例中的测试函数TestCompare中使用了三组预置的测试数据对目标函数strings.Compare进行测试。每次的测试逻辑都比较简单：为被测函数/方法传入预置的测试数据，然后判断被测函数/方法的返回结果是否与预期一致，如果不一致，则测试代码逻辑进入带有testing.Errorf的分支。由此可以得出Go测试代码的一般逻辑，那就是针对给定的输入数据，比较被测函数/方法返回的实际结果值与预期值，如有差异，则通过testing包提供的相关函数输出差异信息。

### 42.2　表驱动的测试实践
o测试代码的逻辑十分简单，约束也甚少，但我们发现：上面仅有三组预置输入数据的示例的测试代码已显得十分冗长，如果为测试预置的数据组数增多，测试函数本身就将变得十分庞大。并且，我们看到上述示例的测试逻辑中存在很多重复的代码，显得十分烦琐。我们来尝试对上述示例做一些改进：

```go
// chapter8/sources/table_driven_strings_test.go
func TestCompare(t *testing.T) {
    compareTests := []struct {
        a, b string
        i    int
    }{
        {"", "", 0},
        {"a", "", 1},
        {"", "a", -1},
    }

    for _, tt := range compareTests {
        cmp := strings.Compare(tt.a, tt.b)
        if cmp != tt.i {
            t.Errorf(`want %v, but Compare(%q, %q) = %v`, tt.i, tt.a, tt.b, cmp)
        }
    }
}
```
在上面这个改进的示例中，我们将之前示例中重复的测试逻辑合并为一个，并将预置的输入数据放入一个自定义结构体类型的切片中。这个示例的长度看似并没有比之前的实例缩减多少，但它却是一个可扩展的测试设计。如果增加输入测试数据的组数，就像下面这样：
```go
// chapter8/sources/table_driven_strings_more_cases_test.go
func TestCompare(t *testing.T) {
    compareTests := []struct {
        a, b string
        i    int
    }{
        {"", "", 0},
        {"a", "", 1},
        {"", "a", -1},
        {"abc", "abc", 0},
        {"ab", "abc", -1},
        {"abc", "ab", 1},
        {"x", "ab", 1},
        {"ab", "x", -1},
        {"x", "a", 1},
        {"b", "x", -1},
        {"abcdefgh", "abcdefgh", 0},
        {"abcdefghi", "abcdefghi", 0},
        {"abcdefghi", "abcdefghj", -1},
    }

    for _, tt := range compareTests {
        cmp := strings.Compare(tt.a, tt.b)
        if cmp != tt.i {
            t.Errorf(`want %v, but Compare(%q, %q) = %v`, tt.i, tt.a, tt.b, cmp)
        }
    }
}
```
可以看到，无须改动后面的测试逻辑，只需在切片中增加数据条目即可。在这种测试设计中，这个自定义结构体类型的切片（上述示例中的compareTests）就是一个表（自定义结构体类型的字段就是列），而基于这个数据表的测试设计和实现则被称为“表驱动的测试”。

### 42.3　表驱动测试的优点
驱动测试本身是编程语言无关的。Go核心团队和Go早期开发者在实践过程中发现表驱动测试十分适合Go代码测试并在标准库和第三方项目中大量使用此种测试设计，这样表驱动测试也就逐渐成为Go的一个惯用法。就像我们从上面的示例中看到的那样，表驱动测试有着诸多优点。

（1）简单和紧凑

从上面的示例中我们看到，表驱动测试将不同测试项经由被测目标执行后的实际输出结果与预期结果的差异判断逻辑合并为一个，这使得测试函数逻辑结构更简单和紧凑。这种简单和紧凑意味着测试代码更容易被开发者理解，因此在测试代码的生命周期内，基于表驱动的测试代码的可维护性更好。

（2）数据即测试

表驱动测试的实质是数据驱动的测试，扩展输入数据集即扩展测试。通过扩展数据集，我们可以很容易地实现提高被测目标测试覆盖率的目的。

（3）结合子测试后，可单独运行某个数据项的测试

我们将表驱动测试与子测试（subtest）结合来改造一下上面的strings_test示例：
```go
// chapter8/sources/table_driven_strings_with_subtest_test.go
func TestCompare(t *testing.T) {
    compareTests := []struct {
        name, a, b string
        i          int
    }{
        {`compareTwoEmptyString`, "", "", 0},
        {`compareSecondParamIsEmpty`, "a", "", 1},
        {`compareFirstParamIsEmpty`, "", "a", -1},
    }

    for _, tt := range compareTests {
        t.Run(tt.name, func(t *testing.T) {
            cmp := strings.Compare(tt.a, tt.b)
            if cmp != tt.i {
                t.Errorf(`want %v, but Compare(%q, %q) = %v`, tt.i, tt.a, tt.b, cmp)
            }
        })
    }
}
```
在示例中，我们将测试结果的判定逻辑放入一个单独的子测试中，这样可以单独执行表中某项数据的测试。比如：我们单独执行表中第一个数据项对应的测试：

```sh
$go test -v  -run /TwoEmptyString table_driven_strings_with_subtest_test.go
=== RUN   TestCompare
=== RUN   TestCompare/compareTwoEmptyString
--- PASS: TestCompare (0.00s)
    --- PASS: TestCompare/compareTwoEmptyString (0.00s)
PASS
ok     command-line-arguments   0.005s
```
综上，建议在编写Go测试代码时优先编写基于表驱动的测试。

### 42.4　表驱动测试实践中的注意事项
#### 1. 表的实现方式

在上面的示例中，测试中使用的表是用自定义结构体的切片实现的，表也可以使用基于自定义结构体的其他集合类型（如map）来实现。我们将上面的例子改造为采用map来实现测试数据表：
```go
// chapter8/sources/table_driven_strings_with_map_test.go
func TestCompare(t *testing.T) {
    compareTests := map[string]struct {
        a, b string
        i    int
    }{
        `compareTwoEmptyString`:     {"", "", 0},
        `compareSecondParamIsEmpty`: {"a", "", 1},
        `compareFirstParamIsEmpty`:  {"", "a", -1},
    }

    for name, tt := range compareTests {
        t.Run(name, func(t *testing.T) {
            cmp := strings.Compare(tt.a, tt.b)
            if cmp != tt.i {
                t.Errorf(`want %v, but Compare(%q, %q) = %v`, tt.i, tt.a, tt.b, cmp)
            }
        })
    }
}
```
不过使用map作为数据表时要注意，表内数据项的测试先后顺序是不确定的。

执行两次上面的示例，得到下面的不同结果：
```sh
// 第一次

$go test -v table_driven_strings_with_map_test.go
=== RUN   TestCompare
=== RUN   TestCompare/compareTwoEmptyString
=== RUN   TestCompare/compareSecondParamIsEmpty
=== RUN   TestCompare/compareFirstParamIsEmpty
--- PASS: TestCompare (0.00s)
    --- PASS: TestCompare/compareTwoEmptyString (0.00s)
    --- PASS: TestCompare/compareSecondParamIsEmpty (0.00s)
    --- PASS: TestCompare/compareFirstParamIsEmpty (0.00s)
PASS
ok         command-line-arguments 0.005s

// 第二次

$go test -v table_driven_strings_with_map_test.go
=== RUN   TestCompare
=== RUN   TestCompare/compareFirstParamIsEmpty
=== RUN   TestCompare/compareTwoEmptyString
=== RUN   TestCompare/compareSecondParamIsEmpty
--- PASS: TestCompare (0.00s)
    --- PASS: TestCompare/compareFirstParamIsEmpty (0.00s)
    --- PASS: TestCompare/compareTwoEmptyString (0.00s)
    --- PASS: TestCompare/compareSecondParamIsEmpty (0.00s)
PASS
ok     command-line-arguments   0.005s
```
在上面两次测试执行的输出结果中，子测试的执行先后次序是不确定的，这是由map类型的自身性质所决定的：对map集合类型进行迭代所返回的集合中的元素顺序是不确定的。

#### 2. 测试失败时的数据项的定位
对于非表驱动的测试，在测试失败时，我们往往通过失败点所在的行数，即可判定究竟是哪块测试代码未通
过：
```sh
$go test -v non_table_driven_strings_test.go
=== RUN   TestCompare
    TestCompare: non_table_driven_strings_test.go:16: want 1,
        but Compare("", "") = 0
--- FAIL: TestCompare (0.00s)
FAIL
FAIL       command-line-arguments 0.005s
FAIL
```
在上面这个测试失败的输出结果中，我们可以直接通过行数（non_table_driven_strings_test.go的第16行）定位问题。但在表驱动的测试中，由于一般情况下表驱动的测试的测试结果成功与否的判定逻辑是共享的，因此再通过行数来定位问题就不可行了，因为无论是表中哪一项导致的测试失败，失败结果中输出的引发错误的行号都是相同的：

```sh
$go test -v table_driven_strings_test.go
=== RUN   TestCompare
    TestCompare: table_driven_strings_test.go:21: want -1, but Compare("", "") = 0
    TestCompare: table_driven_strings_test.go:21: want 6, but Compare("a", "") = 1
--- FAIL: TestCompare (0.00s)
FAIL
FAIL       command-line-arguments 0.005s
FAIL
```
在上面这个测试失败的输出结果中，两个测试失败的输出结果中的行号都是21，这样我们就无法快速定位表中导致测试失败的“元凶”。因此，为了在表测试驱动的测试中快速从输出的结果中定位导致测试失败的表项，我们需要在测试失败的输出结果中输出数据表项的唯一标识。

最简单的方法是通过输出数据表项在数据表中的偏移量来辅助定位“元凶”：
```go
// chapter8/sources/table_driven_strings_by_offset_test.go
func TestCompare(t *testing.T) {
    compareTests := []struct {
        a, b string
        i    int
    }{
        {"", "", 7},
        {"a", "", 6},
        {"", "a", -1},
    }

    for i, tt := range compareTests {
        cmp := strings.Compare(tt.a, tt.b)
        if cmp != tt.i {
            t.Errorf(`[table offset: %v] want %v, but Compare(%q, %q) = %v`, i+1, tt.i, tt.a, tt.b, cmp)
        }
    }
}
```
运行该示例：
```sh
$go test -v table_driven_strings_by_offset_test.go
=== RUN   TestCompare
    TestCompare: table_driven_strings_by_offset_test.go:21: [table offset: 1] want 7, but Compare("", "") = 0
    TestCompare: table_driven_strings_by_offset_test.go:21: [table offset: 2] want 6, but Compare("a", "") = 1
--- FAIL: TestCompare (0.00s)
FAIL
FAIL       command-line-arguments 0.005s
FAIL
```
在上面这个例子中，我们通过在测试结果输出中增加数据项在表中的偏移信息来快速定位问题数据。由于切片的数据项下标从0开始，这里进行了+1处理。

另一个更直观的方式是使用名字来区分不同的数据项：
```go
// chapter8/sources/table_driven_strings_by_name_test.go
func TestCompare(t *testing.T) {
    compareTests := []struct {
        name, a, b string
        i          int
    }{
        {"compareTwoEmptyString", "", "", 7},
        {"compareSecondStringEmpty", "a", "", 6},
        {"compareFirstStringEmpty", "", "a", -1},
    }

    for _, tt := range compareTests {
        cmp := strings.Compare(tt.a, tt.b)
        if cmp != tt.i {
            t.Errorf(`[%s] want %v, but Compare(%q, %q) = %v`, tt.name, tt.i, tt.a, tt.b, cmp)
        }
    }
}
```
运行该示例：
```sh
$go test -v table_driven_strings_by_name_test.go
=== RUN   TestCompare
    TestCompare: table_driven_strings_by_name_test.go:21: [compareTwoEmptyString] want 7, but Compare("", "") = 0
    TestCompare: table_driven_strings_by_name_test.go:21: [compareSecondStringEmpty] want 6, but Compare("a", "") = 1
--- FAIL: TestCompare (0.00s)
FAIL
FAIL       command-line-arguments 0.005s
FAIL
```
在上面这个例子中，我们通过在自定义结构体中添加一个name字段来区分不同数据项，并在测试结果输出该name字段以在测试失败时辅助快速定位问题数据。
#### 3. Errorf还是Fatalf
一般情况下，在表驱动的测试中，数据表中的所有表项共享同一个测试结果的判定逻辑。这样我们需要在Errorf和Fatalf中选择一个来作为测试失败信息的输出途径。前面提到过Errorf不会中断当前的goroutine的执行，即便某个数据项导致了测试失败，测试依旧会继续执行下去，而Fatalf恰好相反，它会终止测试执行。

至于是选择Errorf还是Fatalf并没有固定标准，一般而言，如果一个数据项导致的测试失败不会对后续数据项的测试结果造成影响，那么推荐Errorf，这样可以通过执行一次测试看到所有导致测试失败的数据项；否则，如果数据项导致的测试失败会直接影响到后续数据项的测试结果，那么可以使用Fatalf让测试尽快结束，因为继续执行的测试的意义已经不大了。

小结

在本条中，我们学习了编写Go测试代码的一般逻辑，并给出了编写Go测试代码的最佳实践——基于表驱动测试，以及这种惯例的优点。最后我们了解了实施表驱动测试时需要注意的一些事项。


## 第43条 使用testdata管理测试依赖的外部数据文件
在第41条中，我们提到过测试固件的建立与销毁。测试固件是Go测试执行所需的上下文环境，其中测试依赖的外部数据文件就是一种常见的测试固件（可以理解为静态测试固件，因为无须在测试代码中为其单独编写固件的创建和清理辅助函数）。在一些包含文件I/O的包的测试中，我们经常需要从外部数据文件中加载数据或向外部文件写入结果数据以满足测试固件的需求。

在其他主流编程语言中，如何管理测试依赖的外部数据文件往往是由程序员自行决定的，但Go语言是一门面向软件工程的语言。从工程化的角度出发，Go的设计者们将一些在传统语言中由程序员自身习惯决定的事情一一规范化了，这样可以最大限度地提升程序员间的协作效率。而对测试依赖的外部数据文件的管理就是Go语言在这方面的一个典型例子。在本条中，我们就来看看Go管理测试依赖的外部数据文件所采用的一些惯例和最佳实践。


### 43.1　testdata目录

Go语言规定：Go工具链将忽略名为testdata的目录。这样开发者在编写测试时，就可以在名为testdata的目录下存放和管理测试代码依赖的数据文件。而go test命令在执行时会将被测试程序包源码所在目录设置为其工作目录，这样如果要使用testdata目录下的某个数据文件，我们无须再处理各种恼人的路径问题，而可以直接在测试代码中像下面这样定位到充当测试固件的数据文件：

```go
f, err := os.Open("testdata/data-001.txt")
```
考虑到不同操作系统对路径分隔符定义的差别（Windows下使用反斜线“\”，Linux/macOS下使用斜线“/”），使用下面的方式可以使测试代码更具可移植性：

```go
f, err := os.Open(filepath.Join("testdata", "data-001.txt"))
```

在testdata目录中管理测试依赖的外部数据文件的方式在标准库中有着广泛的应用。在$GOROOT/src路径下（Go 1.14）：
```sh
$find . -name "testdata" -print
./cmd/vet/testdata
./cmd/objdump/testdata
./cmd/asm/internal/asm/testdata
...
./image/testdata
./image/png/testdata
./mime/testdata
./mime/multipart/testdata
./text/template/testdata
./debug/pe/testdata
./debug/macho/testdata
./debug/dwarf/testdata
./debug/gosym/testdata
./debug/plan9obj/testdata
./debug/elf/testdata
```
以image/png/testdata为例，这里存储着png包测试代码用作静态测试固件的外部依赖数据文件：
```sh
$ls
benchGray.png             benchRGB.png                   invalid-palette.png
benchNRGBA-gradient.png   gray-gradient.interlaced.png   invalid-trunc.png
benchNRGBA-opaque.png     gray-gradient.png              invalid-zlib.png
benchPaletted.png         invalid-crc32.png              pngsuite/
benchRGB-interlace.png    invalid-noend.png

$ls testdata/pngsuite
README             basn2c08.png    basn4a16.png    ftbgn3p08.png
README.original    basn2c08.sng    basn4a16.sng    ftbgn3p08.sng
...
basn0g16.sng       basn4a08.sng    ftbgn2c16.sng    ftp1n3p08.sng
```
png包的测试代码将这些数据文件作为输入，并将经过被测函数（如png.Decode等）处理后得到的结果数据与预期数据对比：
```go
// $GOROOT/src/image/png/reader_test.go

var filenames = []string{
    "basn0g01",
    "basn0g01-30",
    "basn0g02",
    ...
}

func TestReader(t *testing.T) {
    names := filenames
    if testing.Short() {
        names = filenamesShort
    }
    for _, fn := range names {
        // 读取.png文件
        img, err := readPNG("testdata/pngsuite/" + fn + ".png")
        if err != nil {
            t.Error(fn, err)
            continue
        }
        ...
        // 比较读取的数据img与预期数据
    }
    ...
}
```
我们还经常将预期结果数据保存在文件中并放置在testdata下，然后在测试代码中将被测对象输出的数据与这些预置在文件中的数据进行比较，一致则测试通过；反之，测试失败。来看一个例子：
```go
// chapter8/sources/testdata-demo1/attendee.go
type Attendee struct {
    Name  string
    Age   int
    Phone string
}

func (a *Attendee) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
    tokens := []xml.Token{}

    tokens = append(tokens, xml.StartElement{
           Name: xml.Name{"", "attendee"}})

    tokens = append(tokens, xml.StartElement{Name: xml.Name{"", "name"}})
    tokens = append(tokens, xml.CharData(a.Name))
    tokens = append(tokens, xml.EndElement{Name: xml.Name{"", "name"}})

    tokens = append(tokens, xml.StartElement{Name: xml.Name{"", "age"}})
    tokens = append(tokens, xml.CharData(strconv.Itoa(a.Age)))
    tokens = append(tokens, xml.EndElement{Name: xml.Name{"", "age"}})

    tokens = append(tokens, xml.StartElement{Name: xml.Name{"", "phone"}})
    tokens = append(tokens, xml.CharData(a.Phone))
    tokens = append(tokens, xml.EndElement{Name: xml.Name{"", "phone"}})

    tokens = append(tokens, xml.StartElement{Name: xml.Name{"", "website"}})
    tokens = append(tokens, xml.CharData("https://www.gophercon.com/speaker/"+
        a.Name))
    tokens = append(tokens, xml.EndElement{Name: xml.Name{"", "website"}})

    tokens = append(tokens, xml.EndElement{Name: xml.Name{"", "attendee"}})

    for _, t := range tokens {
        err := e.EncodeToken(t)
        if err != nil {
            return err
        }
    }

    err := e.Flush()
    if err != nil {
        return err
    }

    return nil
}
```
在attendee包中，我们为Attendee类型实现了MarshalXML方法，进而实现了xml包的Marshaler接口。这样，当我们调用xml包的Marshal或MarshalIndent方法序列化上面的Attendee实例时，我们实现的MarshalXML方法会被调用来对Attendee实例进行xml编码。和默认的XML编码不同的是，在我们实现的MarshalXML方法中，我们会根据Attendee的name字段自动在输出的XML格式数据中增加一个元素：website。

下面就来为Attendee的MarshalXML方法编写测试：
```go
// chapter8/sources/testdata-demo1/attendee_test.go

func TestAttendeeMarshal(t *testing.T) {
    tests := []struct {
        fileName string
        a        Attendee
    }{
        {
            fileName: "attendee1.xml",
            a: Attendee{
                Name:  "robpike",
                Age:   60,
                Phone: "13912345678",
            },
        },
    }

    for _, tt := range tests {
        got, err := xml.MarshalIndent(&tt.a, "", "  ")
        if err != nil {
            t.Fatalf("want nil, got %v", err)
        }

        want, err := ioutil.ReadFile(filepath.Join("testdata", tt.fileName))
        if err != nil {
            t.Fatalf("open file %s failed: %v", tt.fileName, err)
        }

        if !bytes.Equal(got, want) {
            t.Errorf("want %s, got %s", string(want), string(got))
        }
    }
}
```
接下来，我们将预期结果放入testdata/attendee1.xml中：
```xml
// testdata/attendee1.xml
<attendee>
  <name>robpike</name>
  <age>60</age>
  <phone>13912345678</phone>
  <website>https://www.gophercon.com/speaker/robpike</website>
</attendee>
```
执行该测试：
```sh
$go test -v .
=== RUN   TestAttendeeMarshal
--- PASS: TestAttendeeMarshal (0.00s)
PASS
ok         sources/testdata-demo1 0.007s
```
测试通过是预料之中的事情。

### 43.2　golden文件惯用法

在为上面的例子准备预期结果数据文件attendee1.xml时，你可能会有这样的问题：attendee1.xml中的数据从哪里得到？

的确可以根据Attendee的MarshalXML方法的逻辑手动“造”出结果数据，但更快捷的方法是通过代码来得到预期结果。可以通过标准格式化函数输出对Attendee实例进行序列化后的结果。如果这个结果与我们的期望相符，那么就可以将它作为预期结果数据写入attendee1.xml文件中：

```go
got, err := xml.MarshalIndent(&tt.a, "", "  ")
if err != nil {
    ...
}
println(string(got)) // 这里输出XML编码后的结果数据
```
如果仅是将标准输出中符合要求的预期结果数据手动复制到attendee1.xml文件中，那么标准输出中的不可见控制字符很可能会对最终复制的数据造成影响，从而导致测试失败。更有一些被测目标输出的是纯二进制数据，通过手动复制是无法实现预期结果数据文件的制作的。因此，我们还需要通过代码来实现attendee1.xml文件内容的填充，比如：
```go
got, err := xml.MarshalIndent(&tt.a, "", "  ")
if err != nil {
    ...
}
ioutil.WriteFile("testdata/attendee1.xml", got, 0644)
```
题出现了！难道我们还要为每个testdata下面的预期结果文件单独编写一个小型的程序来在测试前写入预期数据？能否把将预期数据采集到文件的过程与测试代码融合到一起呢？Go标准库为我们提供了一种惯用法：golden文件。

将上面的例子改造为采用golden文件模式（将attendee1.xml重命名为attendee1.golden以明示该测试用例采用了golden文件惯用法）：
```go
// chapter8/sources/testdata-demo2/attendee_test.go
...

var update = flag.Bool("update", false, "update .golden files")

func TestAttendeeMarshal(t *testing.T) {
    tests := []struct {
        fileName string
        a        Attendee
    }{
        {
            fileName: "attendee1.golden",
            a: Attendee{
                Name:  "robpike",
                Age:   60,
                Phone: "13912345678",
            },
        },
    }

    for _, tt := range tests {
        got, err := xml.MarshalIndent(&tt.a, "", "  ")
        if err != nil {
            t.Fatalf("want nil, got %v", err)
        }

        golden := filepath.Join("testdata", tt.fileName)
        if *update {
            ioutil.WriteFile(golden, got, 0644)
        }

        want, err := ioutil.ReadFile(golden)
        if err != nil {
            t.Fatalf("open file %s failed: %v", tt.fileName, err)
        }

        if !bytes.Equal(got, want) {
            t.Errorf("want %s, got %s", string(want), string(got))
        }
    }
}
```
在改造后的测试代码中，我们看到新增了一个名为update的变量以及它所控制的golden文件的预期结果数据采集过程：

```go
if *update {
    ioutil.WriteFile(golden, got, 0644)
}
```
这样，当我们执行下面的命令时，测试代码会先将最新的预期结果写入testdata目录下的golden文件中，然后将该结果与从golden文件中读出的结果做比较。
```sh
$go test -v . -update
=== RUN   TestAttendeeMarshal
--- PASS: TestAttendeeMarshal (0.00s)
PASS
ok     sources/testdata-demo2   0.006s
```
显然这样执行的测试是一定会通过的，因为在此次执行中，预期结果数据文件的内容就是通过被测函数刚刚生成的。

但带有-update命令参数的go test命令仅在需要进行预期结果数据采集时才会执行，尤其是在因数据生成逻辑或类型结构定义发生变化，需要重新采集预期结果数据时。比如：我们给上面的Attendee结构体类型增加一个新字段topic，如果不重新采集预期结果数据，那么测试一定是无法通过的。

采用golden文件惯用法后，要格外注意在每次重新采集预期结果后，对golden文件中的数据进行正确性检查，否则很容易出现预期结果数据不正确，但测试依然通过的情况。

小结

在这一条中，我们了解到面向工程的Go语言对测试依赖的外部数据文件的存放位置进行了规范，统一使用testdata目录，开发人员可以采用将预期数据文件放在testdata下的方式为测试提供静态测试固件。而Go golden文件的惯用法实现了testdata目录下测试依赖的预期结果数据文件的数据采集与测试代码的融合。

## 第44条 正确运用fake、stub和mock等辅助单元测试

你不需要一个真实的数据库来满足运行单元测试的需求。

对Go代码进行测试的过程中，除了会遇到上一条中所提到的测试代码对外部文件数据的依赖之外，还会经常面对被测代码对外部业务组件或服务的依赖。此外，越是接近业务层，被测代码对外部组件或服务依赖的可能性越大。比如：
```
被测代码需要连接外部Redis服务；
被测代码依赖一个外部邮件服务器来发送电子邮件；
被测代码需与外部数据库建立连接并进行数据操作；
被测代码使用了某个外部RESTful服务。
```
在生产环境中为运行的业务代码提供其依赖的真实组件或服务是必不可少的，也是相对容易的。但是在开发测试环境中，我们无法像在生产环境中那样，为测试（尤其是单元测试）提供真实运行的外部依赖。这是因为测试（尤其是单元测试）运行在各类开发环境、持续集成或持续交付环境中，我们很难要求所有环境为运行测试而搭建统一版本、统一访问方式、统一行为控制以及保持返回数据一致的真实外部依赖组件或服务。反过来说，为被测对象建立依赖真实外部组件或服务的测试代码是十分不明智的，因为这种测试（尤指单元测试）运行失败的概率要远大于其运行成功的概率，失去了存在的意义。

为了能让对此类被测代码的测试进行下去，我们需要为这些被测代码提供其依赖的外部组件或服务的替身，如图44-1所示。

```
<!-- production -->
biz code ---> external component or service
<!-- test environment -->
biz code ---> fake external component or service
```
显然用于代码测试的“替身”不必与真实组件或服务完全相同，替身只需要提供与真实组件或服务相同的接口，只要被测代码认为它是真实的即可。

替身的概念是在测试驱动编程[1]理论中被提出的。作为测试驱动编程理论的最佳实践，xUnit家族框架将替身的概念在单元测试中应用得淋漓尽致，并总结出多种替身，比如fake、stub、mock等。这些概念及其应用模式被汇集在xUnit Test Patterns[2]一书中，该书已成为试驱动开发和xUnit框架拥趸人手一册的“圣经”。

在本条中，我们就来一起看一下如何将xUnit最佳实践中的fake、stub和mock等概念应用到Go语言单元测试中以简化测试（区别于直接为被测代码建立其依赖的真实外部组件或服务），以及这些概念是如何促进被测代码重构以提升可测试性的。

不过fake、stub、mock等替身概念之间并非泾渭分明的，理解这些概念并清晰区分它们本身就是一道门槛。本条尽量不涉及这些概念间的交集以避免讲解过于琐碎。想要深入了解这些概念间差别的读者可以自行精读xUnit Test Patterns。

```
[1]https://www.agilealliance.org/glossary/tdd
[2]https://book.douban.com/subject/1859393
```

### 44.1　fake：真实组件或服务的简化实现版替身

fake这个单词的中文含义是“伪造的”“假的”“伪装的”等。在这里，fake测试就是指采用真实组件或服务的简化版实现作为替身，以满足被测代码的外部依赖需求。比如：当被测代码需要连接数据库进行相关操作时，虽然我们在开发测试环境中无法提供一个真实的关系数据库来满足测试需求，但是可以基于哈希表实现一个内存版数据库来满足测试代码要求，我们用这样一个伪数据库作为真实数据库的替身，使得测试顺利进行下去。

Go标准库中的$GOROOT/src/database/sql/fakedb_test.go就是一个sql.Driver接口的简化版实现，它可以用来打开一个基于内存的数据库（sql.fakeDB）的连接并操作该内存数据库：

```go
// $GOROOT/src/database/sql/fakedb_test.go
...
type fakeDriver struct {
    mu         sync.Mutex
    openCount  int
    closeCount int
    waitCh     chan struct{}
    waitingCh  chan struct{}
    dbs        map[string]*fakeDB
}
...
var fdriver driver.Driver = &fakeDriver{}
func init() {
    Register("test", fdriver) //将自己作为driver进行了注册
}
...
```
在sql_test.go中，标准库利用上面的fakeDriver进行相关测试：
```go
// $GOROOT/src/database/sql/sql_test.go
func TestUnsupportedOptions(t *testing.T) {
    db := newTestDB(t, "people")
    defer closeDB(t, db)
    _, err := db.BeginTx(context.Background(), &TxOptions{
        Isolation: LevelSerializable, ReadOnly: true,
    })
    if err == nil {
        t.Fatal("expected error when using unsupported options, got nil")
    }
}

const fakeDBName = "foo"

func newTestDB(t testing.TB, name string) *DB {
    return newTestDBConnector(t, &fakeConnector{name: fakeDBName}, name)
}

func newTestDBConnector(t testing.TB, fc *fakeConnector, name string) *DB {
    fc.name = fakeDBName
    db := OpenDB(fc)
    if _, err := db.Exec("WIPE"); err != nil {
        t.Fatalf("exec wipe: %v", err)
    }
    if name == "people" {
        exec(t, db, "CREATE|people|name=string,age=int32,photo=blob,dead=bool,
            bdate=datetime")
        exec(t, db, "INSERT|people|name=Alice,age=?,photo=APHOTO", 1)
        exec(t, db, "INSERT|people|name=Bob,age=?,photo=BPHOTO", 2)
        exec(t, db, "INSERT|people|name=Chris,age=?,photo=CPHOTO,bdate=?", 3, chrisBirthday)
    }
    if name == "magicquery" {
        exec(t, db, "CREATE|magicquery|op=string,millis=int32")
        exec(t, db, "INSERT|magicquery|op=sleep,millis=10")
    }
    return db
}
```
标准库中fakeDriver的这个简化版实现还是比较复杂，我们再来看一个自定义的简单例子来进一步理解fake的概念及其在Go单元测试中的应用。

在这个例子中，被测代码为包mailclient中结构体类型mailClient的方法：ComposeAndSend：
```go
// chapter8/sources/faketest1/mailclient.go

type mailClient struct {
    mlr mailer.Mailer
}

func New(mlr mailer.Mailer) *mailClient {
    return &mailClient{
        mlr: mlr,
    }
}

// 被测方法
func (c *mailClient) ComposeAndSend(subject string,
    destinations []string, body string) (string, error) {
    signTxt := sign.Get()
    newBody := body + "\n" + signTxt

    for _, dest := range destinations {
        err := c.mlr.SendMail(subject, dest, newBody)
        if err != nil {
            return "", err
        }
    }
    return newBody, nil
}
```
可以看到在创建mailClient实例的时候，需要传入一个mailer.Mailer接口变量，该接口定义如下：
```go
// chapter8/sources/faketest1/mailer/mailer.go
type Mailer interface {
    SendMail(subject, destination, body string) error
}
```
ComposeAndSend方法将传入的电子邮件内容（body）与签名（signTxt）编排合并后传给Mailer接口实现者的SendMail方法，由其将邮件发送出去。在生产环境中，mailer.Mailer接口的实现者是要与远程邮件服务器建立连接并通过特定的电子邮件协议（如SMTP）将邮件内容发送出去的。但在单元测试中，我们无法满足被测代码的这个要求，于是我们为mailClient实例提供了两个简化版的实现：fakeOkMailer和fakeFailMailer，前者代表发送成功，后者代表发送失败。代码如下：
```go
// chapter8/sources/faketest1/mailclient_test.go
type fakeOkMailer struct{}
func (m *fakeOkMailer) SendMail(subject string, dest string, body string) error {
    return nil
}

type fakeFailMailer struct{}
func (m *fakeFailMailer) SendMail(subject string, dest string, body string) error {
    return fmt.Errorf("can not reach the mail server of dest [%s]", dest)
}
```
下面就是这两个替身在测试中的使用方法：
```go
// chapter8/sources/faketest1/mailclient_test.go
func TestComposeAndSendOk(t *testing.T) {
    m := &fakeOkMailer{}
    mc := mailclient.New(m)
    _, err := mc.ComposeAndSend("hello, fake test", []string{"xxx@example.com"}, "the test body")
    if err != nil {
        t.Errorf("want nil, got %v", err)
    }
}

func TestComposeAndSendFail(t *testing.T) {
    m := &fakeFailMailer{}
    mc := mailclient.New(m)
    _, err := mc.ComposeAndSend("hello, fake test", []string{"xxx@example.com"}, "the test body")
    if err == nil {
        t.Errorf("want non-nil, got nil")
    }
}
```
我们看到这个测试中mailer.Mailer的fake实现的确很简单，简单到只有一个返回语句。但就这样一个极其简化的实现却满足了对ComposeAndSend方法进行测试的所有需求。

使用fake替身进行测试的最常见理由是在测试环境无法构造被测代码所依赖的外部组件或服务，或者这些组件/服务有副作用。fake替身的实现也有两个极端：要么像标准库fakedb_test.go那样实现一个全功能的简化版内存数据库driver，要么像faketest1例子中那样针对被测代码的调用请求仅返回硬编码的成功或失败。这两种极端实现有一个共同点：并不具备在测试前对返回结果进行预设置的能力。这也是上面例子中我们针对成功和失败两个用例分别实现了一个替身的原因。（如果非要说成功和失败也是预设置的，那么fake替身的预设置能力也仅限于设置单一的返回值，即无论调用多少次，传入什么参数，返回值都是一个。）

### 44.2　stub：对返回结果有一定预设控制能力的替身

stub也是一种替身概念，和fake替身相比，stub替身增强了对替身返回结果的间接控制能力，这种控制可以通过测试前对调用结果预设置来实现。不过，stub替身通常仅针对计划之内的结果进行设置，对计划之外的请求也无能为力。

使用Go标准库net/http/httptest实现的用于测试的Web服务就可以作为一些被测对象所依赖外部服务的stub替身。下面就来看一个这样的例子。

该例子的被测代码为一个获取城市天气的客户端，它通过一个外部的天气服务来获得城市天气数据：
```go
// chapter8/sources/stubtest1/weather_cli.go
type Weather struct {
    City    string `json:"city"`
    Date    string `json:"date"`
    TemP    string `json:"temP"`
    Weather string `json:"weather"`
}

func GetWeatherInfo(addr string, city string) (*Weather, error) {
    url := fmt.Sprintf("%s/weather?city=%s", addr, city)
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("http status code is %d", resp.StatusCode)
    }

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var w Weather
    err = json.Unmarshal(body, &w)
    if err != nil {
        return nil, err
    }

    return &w, nil
}
```
下面是针对GetWeatherInfo函数的测试代码：
```go
// chapter8/sources/stubtest1/weather_cli_test.go
var weatherResp = []Weather{
    {
        City:    "nanning",
        TemP:    "26~33",
        Weather: "rain",
        Date:    "05-04",
    },
    {
        City:    "guiyang",
        TemP:    "25~29",
        Weather: "sunny",
        Date:    "05-04",
    },
    {
        City:    "tianjin",
        TemP:    "20~31",
        Weather: "windy",
        Date:    "05-04",
    },
}

func TestGetWeatherInfoOK(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
        r *http.Request) {
        var data []byte

        if r.URL.EscapedPath() != "/weather" {
            w.WriteHeader(http.StatusForbidden)
        }

        r.ParseForm()
        city := r.Form.Get("city")
        if city == "guiyang" {
            data, _ = json.Marshal(&weatherResp[1])
        }
        if city == "tianjin" {
            data, _ = json.Marshal(&weatherResp[2])
        }
        if city == "nanning" {
            data, _ = json.Marshal(&weatherResp[0])
        }

        w.Write(data)
    }))
    defer ts.Close()
    addr := ts.URL
    city := "guiyang"
    w, err := GetWeatherInfo(addr, city)
    if err != nil {
        t.Fatalf("want nil, got %v", err)
    }
    if w.City != city {
        t.Errorf("want %s, got %s", city, w.City)
    }
    if w.Weather != "sunny" {
        t.Errorf("want %s, got %s", "sunny", w.City)
    }
}
```
在上面的测试代码中，我们使用httptest建立了一个天气服务器替身，被测函数GetWeatherInfo被传入这个构造的替身天气服务器的服务地址，其对外部服务的依赖需求被满足。同时，我们看到该替身具备一定的对服务返回应答结果的控制能力，这种控制通过测试前对返回结果的预设置实现（上面例子中设置了三个城市的天气信息结果）。这种能力可以实现对测试结果判断的控制。

接下来，回到mailclient的例子。之前的示例只聚焦于对Send的测试，而忽略了对Compose的测试。如果要验证邮件内容编排得是否正确，就需要对ComposeAndSend方法的返回结果进行验证。但这里存在一个问题，那就是ComposeAndSend依赖的签名获取方法sign.Get中返回的时间签名是当前时间，这对于测试代码来说就是一个不确定的值，这也直接导致ComposeAndSend的第一个返回值的内容是不确定的。这样一来，我们就无法对Compose部分进行测试。要想让其具备可测性，我们需要对被测代码进行局部重构：可以抽象出一个Signer接口（这样就需要修改创建mailClient的New函数），当然也可以像下面这样提取一个包级函数类型变量（考虑到演示的方便性，这里使用了此种方法，但不代表它比抽象出接口的方法更优）：
```go
// chapter8/sources/stubtest2/mailclient.go
var getSign = sign.Get // 提取一个包级函数类型变量
func (c *mailClient) ComposeAndSend(subject, sender string, destinations []string, body string) (string, error) {
    signTxt := getSign(sender)
    newBody := body + "\n" + signTxt

    for _, dest := range destinations {
        err := c.mlr.SendMail(subject, sender, dest, newBody)
        if err != nil {
            return "", err
        }
    }
    return newBody, nil
}
```
我们看到新版mailclient.go提取了一个名为getSign的函数类型变量，其默认值为sign包的Get函数。同时，为了演示，我们顺便更新了ComposeAndSend的参数列表以及mailer的接口定义，并增加了一个sender参数：
```go
// chapter8/sources/stubtest2/mailer/mailer.go
type Mailer interface {
    SendMail(subject, sender, destination, body string) error
}
```
由于getSign的存在，我们就可以在测试代码中为签名获取函数（sign.Get）建立stub替身了。
```go
// chapter8/sources/stubtest2/mailclient_test.go
var senderSigns = map[string]string{
    "tonybai@example.com":  "I'm a go programmer",
    "jimxu@example.com":    "I'm a java programmer",
    "stevenli@example.com": "I'm a object-c programmer",
}
func TestComposeAndSendWithSign(t *testing.T) {
    old := getSign
    sender := "tonybai@example.com"
    timestamp := "Mon, 04 May 2020 11:46:12 CST"

    getSign = func(sender string) string {
        selfSignTxt := senderSigns[sender]
        return selfSignTxt + "\n" + timestamp
    }

    defer func() {
        getSign = old //测试完毕后，恢复原值
    }()

    m := &fakeOkMailer{}
    mc := New(m)
    body, err := mc.ComposeAndSend("hello, stub test", sender,
        []string{"xxx@example.com"}, "the test body")
    if err != nil {
        t.Errorf("want nil, got %v", err)
    }

    if !strings.Contains(body, timestamp) {
        t.Errorf("the sign of the mail does not contain [%s]", timestamp)
    }

    if !strings.Contains(body, senderSigns[sender]) {
        t.Errorf("the sign of the mail does not contain [%s]", senderSigns [sender])
    }

    sender = "jimxu@example.com"
    body, err = mc.ComposeAndSend("hello, stub test", sender,
           []string{"xxx@example.com"}, "the test body")
    if err != nil {
        t.Errorf("want nil, got %v", err)
    }

    if !strings.Contains(body, senderSigns[sender]) {
        t.Errorf("the sign of the mail does not contain [%s]", senderSigns [sender])
    }
}
```
在新版mailclient_test.go中，我们使用自定义的匿名函数替换了getSign原先的值（通过defer在测试执行后恢复原值）。在新定义的匿名函数中，我们根据传入的sender选择对应的个人签名，并将其与预定义的时间戳组合在一起返回给ComposeAndSend方法。

在这个例子中，我们预置了三个Sender的个人签名，即以这三位sender对ComposeAndSend发起请求，返回的结果都在stub替身的控制范围之内。

在GitHub上有一个名为[gostub](https://github.com/prashantv/gostub)的第三方包可以用于简化stub替身的管理和编写。以上面的例子为例，如果改写为使用gostub的测试，代码如下：
```go
// chapter8/sources/stubtest3/mailclient_test.go
func TestComposeAndSendWithSign(t *testing.T) {
    sender := "tonybai@example.com"
    timestamp := "Mon, 04 May 2020 11:46:12 CST"

    stubs := gostub.Stub(&getSign, func(sender string) string {
        selfSignTxt := senderSigns[sender]
        return selfSignTxt + "\n" + timestamp
    })
    defer stubs.Reset()
    ...
}
```

### 44.3　mock：专用于行为观察和验证的替身
和fake、stub替身相比，mock替身更为强大：它除了能提供测试前的预设置返回结果能力之外，还可以对mock替身对象在测试过程中的行为进行观察和验证。不过相比于前两种替身形式，mock存在应用局限（尤指在Go中）。
```
和前两种替身相比，mock的应用范围要窄很多，只用于实现某接口的实现类型的替身。一般需要通过第三方框架实现mock替身。Go官方维护了一个mock框架——gomock（https://github.com/golang/mock），该框架通过代码生成的方式生成实现某接口的替身类型。
```
mock这个概念相对难于理解，我们通过例子来直观感受一下：将上面例子中的fake替身换为mock替身。首先安装Go官方维护的gomock框架。这个框架分两部分：一部分是用于生成mock替身的mockgen二进制程序，另一部分则是生成的代码所要使用的gomock包。先来安装一下mockgen：
```sh
$go get github.com/golang/mock/mockgen
```
通过上述命令，可将mockgen安装到$GOPATH/bin目录下（确保该目录已配置在PATH环境变量中）。

接下来，改造一下mocktest/mailer/mailer.go源码。在源码文件开始处加入go generate命令指示符：

```go
// chapter8/sources/mocktest/mailer/mailer.go
//go:generate mockgen -source=./mailer.go -destination=./mock_mailer.go -package=mailer Mailer

package mailer

type Mailer interface {
    SendMail(subject, sender, destination, body string) error
}
```
接下来，在mocktest目录下，执行go generate命令以生成mailer.Mailer接口实现的替身。执行完go generate命令后，我们会在mocktest/mailer目录下看到一个新文件——mock_mailer.go：
```go
// chapter8/sources/mocktest/mailer/mock_mailer.go

// Code generated by MockGen. DO NOT EDIT.
// Source: ./mailer.go

// mailer包是一个自动生成的 GoMock包
package mailer

import (
    gomock "github.com/golang/mock/gomock"
    reflect "reflect"
)

// MockMailer是Mailer接口的一个模拟实现
type MockMailer struct {
    ctrl     *gomock.Controller
    recorder *MockMailerMockRecorder
}

// MockMailerMockRecorder 是 MockMailer的模拟recorder
type MockMailerMockRecorder struct {
    mock *MockMailer
}

// NewMockMailer创建一个新的模拟实例
func NewMockMailer(ctrl *gomock.Controller) *MockMailer {
    mock := &MockMailer{ctrl: ctrl}
    mock.recorder = &MockMailerMockRecorder{mock}
    return mock
}

// EXPECT返回一个对象，允许调用者指示预期的使用情况
func (m *MockMailer) EXPECT() *MockMailerMockRecorder {
    return m.recorder
}

// SendMail模拟基本方法
func (m *MockMailer) SendMail(subject, sender, destination, body string) error {
    m.ctrl.T.Helper()
    ret := m.ctrl.Call(m, "SendMail", subject, sender, destination, body)
    ret0, _ := ret[0].(error)
    return ret0
}

// SendMail表示预期的对SendMail的调用
func (mr *MockMailerMockRecorder) SendMail(subject, sender, destination, body interface{}) *gomock.Call {
    mr.mock.ctrl.T.Helper()
    return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMail", reflect.TypeOf((*MockMailer)(nil).SendMail), subject, sender, destination, body)
}
```
有了替身之后，我们就以将其用于对ComposeAndSend方法的测试了。下面是使用了mock替身的mailclient_test.go：
```go
// chapter8/sources/mocktest/mocktest/mailclient_test.go
package mailclient

import (
    "errors"
    "testing"

    "github.com/bigwhite/mailclient/mailer"
    "github.com/golang/mock/gomock"
)

var senderSigns = map[string]string{
    "tonybai@example.com":  "I'm a go programmer",
    "jimxu@example.com":    "I'm a java programmer",
    "stevenli@example.com": "I'm a object-c programmer",
}

func TestComposeAndSendOk(t *testing.T) {
    old := getSign
    sender := "tonybai@example.com"
    timestamp := "Mon, 04 May 2020 11:46:12 CST"

    getSign = func(sender string) string {
        selfSignTxt := senderSigns[sender]
        return selfSignTxt + "\n" + timestamp
    }
    defer func() {
        getSign = old //测试完毕后，恢复原值
    }()

    mockCtrl := gomock.NewController(t)
    defer mockCtrl.Finish() //Go 1.14及之后版本中无须调用该Finish

    mockMailer := mailer.NewMockMailer(mockCtrl)
    mockMailer.EXPECT().SendMail("hello, mock test", sender,
     "dest1@example.com",
     "the test body\n"+senderSigns[sender]+"\n"+timestamp).Return(nil).Times(1)
    mockMailer.EXPECT().SendMail("hello, mock test", sender,
     "dest2@example.com",
     "the test body\n"+senderSigns[sender]+"\n"+timestamp).Return(nil).Times(1)

    mc := New(mockMailer)
    _, err := mc.ComposeAndSend("hello, mock test",
      sender, []string{"dest1@example.com", "dest2@example.com"}, "the test body")
    if err != nil {
        t.Errorf("want nil, got %v", err)
    }
}
...
```
上面这段代码的重点在于下面这几行：
```go
mockMailer.EXPECT().SendMail("hello, mock test", sender,
    "dest1@example.com",
    "the test body\n"+senderSigns[sender]+"\n"+timestamp).Return(nil).Times(1)
```
这就是前面提到的mock替身具备的能力：在测试前对预期返回结果进行设置（这里设置SendMail返回nil），对替身在测试过程中的行为进行验证。Times(1)意味着以该参数列表调用的SendMail方法在测试过程中仅被调用一次，多一次调用或没有调用均会导致测试失败。这种对替身观察和验证的能力是mock区别于stub的重要特征。

gomock是一个通用的mock框架，社区还有一些专用的mock框架可用于快速创建mock替身，比如：[go-sqlmock](https://github.com/DATA-DOG/go-sqlmock)专门用于创建sql/driver包中的Driver接口实现的mock替身，可以帮助Gopher简单、快速地建立起对数据库操作相关方法的单元测试。

小结

本条介绍了当被测代码对外部组件或服务有强依赖时可以采用的测试方案，这些方案采用了相同的思路：为这些被依赖的外部组件或服务建立替身。这里介绍了三类替身以及它们的适用场合与注意事项。

本条要点如下。

fake、stub、mock等替身概念之间并非泾渭分明的，对这些概念的理解容易混淆。比如标准库net/http/transfer_test.go文件中的mockTransferWriter类型，虽然其名字中带有mock，但实质上它更像是一个fake替身。

我们更多在包内测试应用上述替身概念辅助测试，这就意味着此类测试与被测代码是实现级别耦合的，这样的测试健壮性较差，一旦被测代码内部逻辑有变化，测试极容易失败。通过fake、stub、mock等概念实现的替身参与的测试毕竟是在一个虚拟的“沙箱”环境中，不能代替与真实依赖连接的测试，因此，在集成测试或系统测试等使用真实外部组件或服务的测试阶段，务必包含与真实依赖的联测用例。

```
fake替身主要用于被测代码依赖组件或服务的简化实现。
stub替身具有有限范围的、在测试前预置返回结果的控制能力。
mock替身则专用于对替身的行为进行观察和验证的测试，一般用作Go接口类型的实现的替身。
```

## 第45条 使用模糊测试让潜在bug无处遁形

在Go 1.5版本发布的同时，前英特尔黑带级工程师、现谷歌工程师Dmitry Vyukov发布了Go语言模糊测试工具go-fuzz。在GopherCon 2015技术大会上，Dmitry Vyukov在其名为“GoDynamic Tools”的主题演讲中着重介绍了go-fuzz。

对于模糊测试（fuzz testing），想必很多Gopher比较陌生，当初笔者也不例外，至少在接触go-fuzz之前，笔者从未在Go或其他编程语言中使用过类似的测试工具。根据维基百科的定义，模糊测试就是指半自动或自动地为程序提供非法的、非预期、随机的数据，并监控程序在这些输入数据下是否会出现崩溃、内置断言失败、内存泄露、安全漏洞等情况（见图45-1）。

模糊测试始于1988年Barton Miller所做的一项有关Unix随机测试的项目。到目前为止，已经有许多有关模糊测试的理论支撑，并且越来越多的编程语言开始提供对模糊测试的支持，比如在编译器层面原生提供模糊测试支持的LLVM fuzzer项目libfuzzer、历史最悠久的面向安全的fuzzer方案afl-fuzz、谷歌开源的面向可伸缩模糊测试基础设施的ClusterFuzz等。

传统软件测试技术越来越无法满足现代软件日益增长的规模、复杂性以及对开发速度的要求。传统软件测试一般会针对被测目标的特性进行人工测试设计。在设计一些异常测试用例的时候，测试用例质量好坏往往取决于测试设计人员对被测系统的理解程度及其个人能力。即便测试设计人员个人能力很强，对被测系统也有较深入的理解，他也很难在有限的时间内想到所有可能的异常组合和异常输入，尤其是面对庞大的分布式系统的时候。系统涉及的自身服务组件、中间件、第三方系统等多且复杂，这些系统中的潜在bug或者组合后形成的潜在bug是我们无法预知的。而将随机测试、边界测试、试探性攻击等测试技术集于一身的模糊测试对于上述传统测试技术存在的问题是一个很好的补充和解决方案。

在本条中，我们就来看看如何在Go中为被测代码建立起模糊测试，让那些潜在bug无处遁形。

### 45.1　模糊测试在挖掘Go代码的潜在bug中的作用

go-fuzz工具让Gopher具备了在Go语言中为被测代码建立模糊测试的条件。但模糊测试在挖掘Go代码中潜在bug中的作用究竟有多大呢？我们可以从Dmitry Vyukov提供的一组数据中看出来。

Dmitry Vyukov使用go-fuzz对当时（2015年）的Go标准库以及其他第三方开源库进行了模糊测试并取得了惊人的战果：

```sh
// 60个测试
60 tests

// 在Go标准库中发现137个bug(70个已经修复)
137 bugs in std lib (70 fixed)

// 在其他项目中发现165个bug
165 elsewhere (47 in gccgo, 30 in golang.org/x, 42 in freetype-go, protobuf, http2,
    bson)
```
go-fuzz的战绩在持续扩大，截至本书写作时，列在go-fuzz官方站点上的、由广大Gopher分享出来的已发现bug已有近400个，未分享出来的通过go-fuzz发现的bug估计远远不止这个数量。

