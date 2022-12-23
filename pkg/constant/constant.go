package constant

import "fmt"

// 下面的变量将在编译时被命令行参数覆盖
var (
	Version   string
	Revision  string
	BuildTime string
)

func FullVersion() string {
	return fmt.Sprintf("%s %s %s", Version, Revision, BuildTime)
}

const (
	EnableLog           = true
	LogOutputToTerminal = false
)
const (
	BridgeName = "blitz0"
	Mtu        = 1500
	VxlanId    = 666
	VXLANPort  = 12564
	VXLANGroup = "239.1.1.1"
	VXLANName  = "blitznet"
)
