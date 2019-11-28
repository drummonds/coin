package coin

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

type Price struct {
	Commodity *Commodity
	Currency  *Commodity
	Value     *Amount
	Time      time.Time

	CommodityId string
	currencyId  string
}

var Prices []*Price

func (p *Price) Write(w io.Writer, ledger bool) error {
	date := p.Time.Format(DateFormat)
	_, err := io.WriteString(w, "P "+date+" "+p.Commodity.SafeId(ledger)+" ")
	if err != nil {
		return err
	}
	err = p.Value.Write(w, ledger)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	return err
}

var priceRE = regexp.MustCompile(`P ` + DateRE + `\s+` + CommodityRE + `\s+` + AmountRE)

func (p *Parser) parsePrice() (*Price, error) {
	match := priceRE.FindSubmatch(p.Bytes())
	if match == nil {
		return nil, fmt.Errorf("Invalid price line")
	}
	date := mustParseDate(match[1])
	currencyId := string(match[5])
	c := MustFindCommodity(currencyId)
	amt, err := parseAmount(match[3], c)
	if err != nil {
		return nil, err
	}
	return &Price{
		Time:        date,
		Value:       amt,
		CommodityId: string(match[2]),
		currencyId:  currencyId,
	}, nil
}

func (p *Price) String() string {
	var b strings.Builder
	p.Write(&b, false)
	return b.String()
}
