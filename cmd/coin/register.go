package main

import (
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mkobetic/coin"
	"github.com/mkobetic/coin/check"
)

func init() {
	(&cmdRegister{}).newCommand("register", "reg", "r")
}

type cmdRegister struct {
	*flag.FlagSet
	verbose                 bool
	recurse                 bool
	begin, end              coin.Date
	weekly, monthly, yearly bool
	top                     int
	cumulative              bool
	maxLabelWidth           int
}

func (_ *cmdRegister) newCommand(names ...string) command {
	var cmd cmdRegister
	cmd.FlagSet = newCommand(&cmd, names...)
	cmd.BoolVar(&cmd.verbose, "v", false, "log debug info to stderr")
	cmd.BoolVar(&cmd.recurse, "r", false, "include children account postings")
	cmd.Var(&cmd.begin, "b", "begin register from this date")
	cmd.Var(&cmd.end, "e", "end register on this date")
	cmd.BoolVar(&cmd.weekly, "w", false, "aggregate postings by week")
	cmd.BoolVar(&cmd.monthly, "m", false, "aggregate postings by month")
	cmd.BoolVar(&cmd.yearly, "y", false, "aggregate postings by year")
	cmd.IntVar(&cmd.top, "t", 5, "include this many subaccounts in aggregate results")
	cmd.BoolVar(&cmd.cumulative, "c", false, "aggregate cumulatively across time")
	cmd.IntVar(&cmd.maxLabelWidth, "l", 12, "maximum width of a column label")
	return &cmd
}

func (cmd *cmdRegister) init() {
	check.If(cmd.NArg() > 0, "account filter is required")
	coin.LoadAll()
}
func (cmd *cmdRegister) execute(f io.Writer) {
	pattern := cmd.Arg(0)
	acc := coin.MustFindAccount(pattern)
	fmt.Fprintln(f, acc.FullName, acc.Commodity.Id)
	if cmd.recurse {
		cmd.recursiveRegister(f, acc)
	} else {
		cmd.flatRegister(f, acc)
	}
}

func (cmd *cmdRegister) flatRegister(f io.Writer, acc *coin.Account) {
	postings := cmd.trim(acc.Postings)
	if len(postings) == 0 {
		return
	}
	switch {
	case cmd.weekly:
		cmd.debugf("aggregating weekly")
		cmd.flatRegisterAggregated(f, postings, acc.Commodity, week, coin.DateFormat)
	case cmd.monthly:
		cmd.debugf("aggregating monthly")
		cmd.flatRegisterAggregated(f, postings, acc.Commodity, month, coin.MonthFormat)
	case cmd.yearly:
		cmd.debugf("aggregating yearly")
		cmd.flatRegisterAggregated(f, postings, acc.Commodity, year, coin.YearFormat)
	default:
		cmd.flatRegisterFull(f, postings, acc.Commodity)
	}
}

func (cmd *cmdRegister) flatRegisterAggregated(f io.Writer,
	postings []*coin.Posting,
	commodity *coin.Commodity,
	by func(time.Time) time.Time,
	format string,
) {
	totals := &totals{by: by}
	for _, p := range postings {
		totals.add(p.Transaction.Posted, p.Quantity)
	}
	total := coin.NewAmount(big.NewInt(0), commodity)
	for _, t := range totals.all {
		total.AddIn(t.Amount)
		fmt.Fprintf(f, "%s | %12a | %12a\n",
			t.Time.Format(format),
			t.Amount,
			total,
		)
	}
}

func (cmd *cmdRegister) flatRegisterFull(f io.Writer, postings []*coin.Posting, commodity *coin.Commodity) {
	var desc, acct int
	for _, s := range postings {
		desc = max(desc, len(s.Transaction.Description))
		acct = max(acct, len(s.Transaction.Other(s).Account.FullName))
	}
	var total = coin.NewAmount(big.NewInt(0), commodity)
	for _, s := range postings {
		total.AddIn(s.Quantity)
		fmt.Fprintf(f, "%s | %*s | %*s | %10a | %10a\n",
			s.Transaction.Posted.Format(coin.DateFormat),
			min(desc, 50),
			s.Transaction.Description,
			min(acct, 50),
			s.Transaction.Other(s).Account.FullName,
			s.Quantity,
			total,
		)
	}
}

func (cmd *cmdRegister) recursiveRegister(f io.Writer, acc *coin.Account) {
	switch {
	case cmd.weekly:
		cmd.debugf("aggregating weekly")
		cmd.recursiveRegisterAggregated(f, acc, week, coin.DateFormat)
	case cmd.monthly:
		cmd.debugf("aggregating monthly")
		cmd.recursiveRegisterAggregated(f, acc, month, coin.MonthFormat)
	case cmd.yearly:
		cmd.debugf("aggregating yearly")
		cmd.recursiveRegisterAggregated(f, acc, year, coin.YearFormat)
	default:
		cmd.recursiveRegisterFull(f, acc)
	}
}

func (cmd *cmdRegister) recursiveRegisterAggregated(f io.Writer,
	acc *coin.Account,
	by func(time.Time) time.Time,
	format string,
) {
	totals := accountTotals{}
	acc.WithChildrenDo(func(a *coin.Account) {
		ts := totals.newTotals(a, by, cmd.cumulative)
		for _, p := range cmd.trim(a.Postings) {
			ts.add(p.Transaction.Posted, p.Quantity)
		}
	})
	acc.FirstWithChildrenDo(func(a *coin.Account) {
		child := totals[a]
		parent := totals[a.Parent]
		if parent != nil {
			parent.merge(child)
		}
	})
	totals.sanitize()
	accTotals := totals[acc]
	delete(totals, acc)
	var accounts []*coin.Account
	totals, accounts = totals.top(cmd.top)
	totals.mergeTime(accTotals)
	totals[acc] = accTotals
	accounts = append(accounts, acc)
	if cmd.cumulative {
		totals.makeCumulative()
	}
	label := func(a *coin.Account) string {
		switch a {
		case nil:
			return "Other"
		case acc:
			return "Totals"
		default:
			n := strings.TrimPrefix(a.FullName, acc.FullName)
			return coin.ShortenAccountName(n, cmd.maxLabelWidth)
		}
	}
	totals.print(f, accounts, label, format)
}

func (cmd *cmdRegister) recursiveRegisterFull(f io.Writer, acc *coin.Account) {
	var postings []*coin.Posting
	var desc, acct, from int
	prefix := acc.FullName
	acc.WithChildrenDo(func(a *coin.Account) {
		if l := len(a.FullName) - len(acc.FullName); l > from {
			from = l
		}
		for _, s := range cmd.trim(a.Postings) {
			desc = max(desc, len(s.Transaction.Description))
			acct = max(acct, len(strings.TrimPrefix(s.Transaction.Other(s).Account.FullName, prefix)))
			postings = append(postings, s)
		}
	})
	// sort all postings by time
	sort.SliceStable(postings, func(i, j int) bool {
		return postings[i].Transaction.Posted.Before(postings[j].Transaction.Posted)
	})
	for _, s := range postings {
		fmt.Fprintf(f, "%s | %*s | %*s | %*s | %10a%s\n",
			s.Transaction.Posted.Format(coin.DateFormat),
			min(desc, 50),
			s.Transaction.Description,
			min(from, 50),
			strings.TrimPrefix(s.Account.FullName, prefix),
			min(acct, 50),
			strings.TrimPrefix(s.Transaction.Other(s).Account.FullName, prefix),
			s.Quantity,
			s.Account.CommodityId,
		)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (cmd *cmdRegister) trim(postings []*coin.Posting) []*coin.Posting {
	if !cmd.begin.IsZero() {
		from := sort.Search(len(postings), func(i int) bool {
			return !postings[i].Transaction.Posted.Before(cmd.begin.Time)
		})
		if from == len(postings) {
			return nil
		}
		postings = postings[from:]
	}
	if !cmd.end.IsZero() {
		to := sort.Search(len(postings), func(i int) bool {
			return !postings[i].Transaction.Posted.Before(cmd.end.Time)
		})
		if to == len(postings) {
			return postings
		}
		postings = postings[:to]
	}
	return postings
}

func (cmd *cmdRegister) debugf(format string, args ...interface{}) {
	if !cmd.verbose {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
}
