package revolut

import (
	"encoding/csv"
	"io"
	"log"
	"strings"
	"time"

	"github.com/fiscafacile/CryptoFiscaFacile/wallet"
	"github.com/shopspring/decimal"
)

type CsvTX struct {
	Timestamp   time.Time
	Description string
	Rate        decimal.Decimal
	PaidOut     decimal.Decimal
	PaidIn      decimal.Decimal
	ExchangeOut wallet.Currency
	ExchangeIn  string
	Balance     decimal.Decimal
	Category    string
	Notes       string
}

func (revo *Revolut) ParseCSV(reader io.Reader) (err error) {
	csvReader := csv.NewReader(reader)
	records, err := csvReader.ReadAll()
	if err == nil {
		var curr string
		for _, r := range records {
			if r[0] == "Completed Date" {
				curr = strings.Split(r[2], "(")[1]
				curr = strings.Split(curr, ")")[0]
			} else {
				tx := CsvTX{}
				tx.Timestamp, err = time.Parse("2 Jan 2006", f2e(r[0]))
				if err != nil {
					log.Println("Error Parsing Timestamp :", r[0])
				}
				tx.Description = strings.ReplaceAll(r[1], "\u00a0", "")
				// spew.Dump(strings.Split(r[1], " "))
				tx.Rate, err = decimal.NewFromString(strings.Split(r[1], " ")[7][:9])
				if err != nil {
					log.Println("Error Parsing Rate :", strings.Split(r[1], " ")[7][:9])
				}
				if r[2] != "" {
					tx.PaidOut, err = decimal.NewFromString(r[2])
					if err != nil {
						log.Println("Error Parsing PaidOut :", r[2])
					}
				} else {
					tx.PaidIn, err = decimal.NewFromString(r[3])
					if err != nil {
						log.Println("Error Parsing PaidIn :", r[3])
					}
				}
				s := strings.Split(r[4], " ")
				tx.ExchangeOut.Code = s[0]
				tx.ExchangeOut.Amount, err = decimal.NewFromString(s[1])
				if err != nil {
					log.Println("Error Parsing ExchangeOut.Amount :", s[1])
				}
				tx.ExchangeIn = r[5]
				tx.Balance, err = decimal.NewFromString(r[6])
				if err != nil {
					log.Println("Error Parsing Balance :", r[6])
				}
				tx.Category = r[7]
				tx.Notes = r[8]
				revo.CsvTXs = append(revo.CsvTXs, tx)
				// Fill TXsByCategory
				if !tx.PaidIn.IsZero() {
					t := wallet.TX{Timestamp: tx.Timestamp, Note: "Revolut CSV : " + tx.Description}
					t.Items = make(map[string][]wallet.Currency)
					t.Items["To"] = append(t.Items["To"], wallet.Currency{Code: curr, Amount: tx.PaidIn})
					t.Items["From"] = append(t.Items["From"], tx.ExchangeOut)
					revo.TXsByCategory["Exchanges"] = append(revo.TXsByCategory["Exchanges"], t)
				} else if !tx.PaidOut.IsZero() {
					t := wallet.TX{Timestamp: tx.Timestamp, Note: "Revolut CSV : " + tx.Description}
					t.Items = make(map[string][]wallet.Currency)
					t.Items["From"] = append(t.Items["From"], wallet.Currency{Code: curr, Amount: tx.PaidOut})
					t.Items["To"] = append(t.Items["To"], wallet.Currency{Code: "EUR", Amount: tx.PaidOut.Mul(tx.Rate)})
					revo.TXsByCategory["Exchanges"] = append(revo.TXsByCategory["Exchanges"], t)
				} else {
					log.Println("Unmanaged ", tx)
				}
			}
		}
	}
	return
}

func f2e(french string) (english string) {
	english = strings.ReplaceAll(french, "jan.", "Jan")
	english = strings.ReplaceAll(english, "févr.", "Feb")
	english = strings.ReplaceAll(english, "mars", "Mar")
	english = strings.ReplaceAll(english, "avr.", "Apr")
	english = strings.ReplaceAll(english, "mai", "May")
	english = strings.ReplaceAll(english, "juin", "June")
	english = strings.ReplaceAll(english, "juil.", "July")
	english = strings.ReplaceAll(english, "août", "Aug")
	english = strings.ReplaceAll(english, "sept.", "Sep")
	english = strings.ReplaceAll(english, "oct.", "Oct")
	english = strings.ReplaceAll(english, "nov.", "Nov")
	english = strings.ReplaceAll(english, "déc.", "Dec")
	return
}