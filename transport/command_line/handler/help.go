package handler

import (
	"e-commerce_order_analytics_system/transport/command_line/lib"
	"fmt"
	"io"
)

func (cli *commandHandler) CmdHelp(out io.Writer) int {
	fmt.Fprint(out, lib.USAGE)
	return 0
}
