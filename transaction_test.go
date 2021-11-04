package coin

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mkobetic/coin/assert"
)

func Test_ParseTransaction(t *testing.T) {
	r := strings.NewReader(`
2008/04/02 COSTCO WHOLESALE
  Expenses:Groceries       37.92 CAD
  Liabilities:Credit:AMEX  -37.92 CAD

2008/04/03
  Assets:Investments:Martin:RRSP:Archive:TDB162  462.250 TDB162
  Assets:Investments:Martin:RRSP:CAD             -6000.00 CAD

2008/04/03 [PR]PC #7959 BELL'S
  Expenses:Groceries   125.66 CAD
  Assets:Bank:Checking  -125.66 CAD
`)
	p := NewParser(r)
	i, err := p.Next("")
	assert.NoError(t, err)
	tr, ok := i.(*Transaction)
	assert.Equal(t, ok, true)
	assert.Equal(t, tr.Description, "COSTCO WHOLESALE")
	assert.Equal(t, len(tr.Postings), 2)

	i, err = p.Next("")
	assert.NoError(t, err)
	tr, ok = i.(*Transaction)
	assert.Equal(t, ok, true)
	assert.Equal(t, tr.Description, "")
	assert.Equal(t, len(tr.Postings), 2)

	i, err = p.Next("")
	assert.NoError(t, err)
	tr, ok = i.(*Transaction)
	assert.Equal(t, ok, true)
	assert.Equal(t, tr.Description, "[PR]PC #7959 BELL'S")
	assert.Equal(t, len(tr.Postings), 2)

}

func Test_ParseTransactionNotes(t *testing.T) {
	r := strings.NewReader(`
; first example
; with a short note
2018/10/01 payee1 ; hello
  AA 10.00 CAD
    ; single-line split note
  BB -10.00 CAD

; multi-line note
2018/10/02 payee2
  ; hello world
  ;   and again
  ;and again
  AA 50.00 CAD
  BB 50.00 CAD
  CC -100.00 CAD
	; multi-line
  ; split
    ;note
`)
	p := NewParser(r)
	i, err := p.Next("")
	assert.NoError(t, err)
	tr, ok := i.(*Transaction)
	assert.Equal(t, ok, true)
	assert.Equal(t, tr.Description, "payee1")
	assert.Equal(t, tr.Note, "hello")
	assert.Equal(t, len(tr.Postings), 2)
	assert.Equal(t, tr.Postings[0].Note, "single-line split note")

	i, err = p.Next("")
	assert.NoError(t, err)
	tr, ok = i.(*Transaction)
	assert.Equal(t, ok, true)
	assert.Equal(t, tr.Description, "payee2")
	assert.Equal(t, tr.Note, "hello world\n  and again\nand again")
	assert.Equal(t, len(tr.Postings), 3)
	assert.Equal(t, tr.Postings[2].Note, "multi-line\nsplit\nnote")
}

func Test_ParseTransactionCode(t *testing.T) {
	r := strings.NewReader(`
2018/10/01 (code) payee1 ; hello
  AA 10.00 CAD
  BB -10.00 CAD
`)
	p := NewParser(r)
	i, err := p.Next("")
	assert.NoError(t, err)
	tr, ok := i.(*Transaction)
	assert.Equal(t, ok, true)
	assert.Equal(t, tr.Description, "payee1")
	assert.Equal(t, tr.Code, "code")
	assert.Equal(t, tr.Note, "hello")
	assert.Equal(t, len(tr.Postings), 2)
}

func Test_ParseTransactionBalance(t *testing.T) {
	r := strings.NewReader(`
2018/10/01 payee1
  AA 10.00 CAD
  BB -10.00 CAD = 50.00 CAD
`)
	p := NewParser(r)
	i, err := p.Next("")
	assert.NoError(t, err)
	tr, ok := i.(*Transaction)
	assert.Equal(t, ok, true)
	assert.Equal(t, tr.Description, "payee1")
	assert.Equal(t, len(tr.Postings), 2)
	assert.Equal(t, fmt.Sprintf("%a", tr.Postings[1].Balance), "50.00")
}

func Test_TransactionsByTimeDay(t *testing.T) {
	Year, Month, Day = 2000, 5, 7
	var transactions TransactionsByTime
	for _, days := range []int{0, 0, 0, 1, 4, 4, 4, 7, 13, 13, 20} {
		match := DateREX.Match([]byte(fmt.Sprintf("+%dd", days)))
		posted := mustParseDate(match, 0)
		transactions = append(transactions, &Transaction{Posted: posted})
	}
	check := func(t *testing.T, day time.Time, count int) {
		transactions := transactions.Day(day)
		assert.Equal(t, len(transactions), count, "%v length mismatch", day)
		for i, tr := range transactions {
			assert.Equal(t, tr.Posted, day, "%v transaction %d posted date mismatch", day, i)
		}
	}
	check(t, transactions[0].Posted.AddDate(0, 0, -10), 0)
	check(t, transactions[0].Posted.AddDate(0, 0, -1), 0)
	check(t, transactions[0].Posted, 3)
	check(t, transactions[3].Posted, 1)
	check(t, transactions[3].Posted.AddDate(0, 0, 1), 0)
	check(t, transactions[4].Posted, 3)
	check(t, transactions[7].Posted, 1)
	check(t, transactions[8].Posted, 2)
	check(t, transactions[10].Posted, 1)
	check(t, transactions[10].Posted.AddDate(0, 0, 1), 0)
	check(t, transactions[10].Posted.AddDate(0, 0, 10), 0)
}
