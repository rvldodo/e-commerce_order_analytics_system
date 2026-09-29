package handler

import (
	"e-commerce_order_analytics_system/internal/usecase/report"
	"fmt"
	"io"
	"text/tabwriter"
)

func (cli *commandHandler) CmdTypes(out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tFLAGS\tDESCRIPTION")
	for _, t := range report.Types {
		flags := "-"
		switch {
		case t.UsesYear:
			flags = "--year"
		case t.UsesDays:
			flags = "--days"
		case t.UsesDate:
			flags = "--date"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", t.Type, flags, t.Description)
	}
	return w.Flush()
}
