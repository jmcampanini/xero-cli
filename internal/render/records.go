package render

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/jmcampanini/xero-cli/internal/xero"
)

func yesNo(value string) string {
	switch value {
	case "true":
		return "yes"
	case "false":
		return "no"
	default:
		return value
	}
}

func accountLabel(record xero.Record) string {
	return strings.TrimSpace(record.Text("Code") + " " + record.Text("Name"))
}

func nameWithID(name, id string) string {
	if name == "" {
		return id
	}
	if id == "" {
		return name
	}
	return name + " (" + id + ")"
}

func trackingLabel(items []xero.Record) string {
	var labels []string
	for _, item := range items {
		category, option := item.Text("Name"), item.Text("Option")
		if category == "" {
			category = item.Text("TrackingCategoryName")
		}
		if option == "" {
			option = item.Text("TrackingOptionName")
		}
		labels = append(labels, category+"="+option)
	}
	return strings.Join(labels, "; ")
}

// Records renders document collections only after every row has been prepared.
func Records(w io.Writer, org xero.Identity, resource string, items []xero.Record, color string) error {
	var header []string
	var rows [][]string
	for _, item := range items {
		switch resource {
		case "BankTransactions":
			kind := item.Text("Type")
			if item.Text("Status") == "DELETED" {
				kind += " (deleted)"
			}
			rows = append(rows, []string{item.Text("Date"), kind, item.Object("Contact").Text("Name"), item.Text("Reference"), item.Text("Total"), yesNo(item.Text("IsReconciled")), item.Text("BankTransactionID")})
		case "BankTransfers":
			rows = append(rows, []string{item.Text("Date"), accountLabel(item.Object("FromBankAccount")), accountLabel(item.Object("ToBankAccount")), item.Text("Amount"), item.Text("Reference"), yesNo(item.Text("FromIsReconciled")) + "/" + yesNo(item.Text("ToIsReconciled")), item.Text("BankTransferID")})
		case "ManualJournals":
			debits, err := xero.JournalDebits(item.Records("JournalLines"))
			if err != nil {
				return err
			}
			rows = append(rows, []string{item.Text("Date"), item.Text("Status"), item.Text("Narration"), debits, yesNo(item.Text("ShowOnCashBasisReports")), item.Text("ManualJournalID")})
		case "Contacts":
			rows = append(rows, []string{item.Text("Name"), item.Text("EmailAddress"), yesNo(item.Text("IsCustomer")), yesNo(item.Text("IsSupplier")), item.Text("ContactStatus"), item.Text("ContactID")})
		}
	}
	switch resource {
	case "BankTransactions":
		header = []string{"DATE", "TYPE", "CONTACT", "REFERENCE", "TOTAL", "RECONCILED", "ID"}
	case "BankTransfers":
		header = []string{"DATE", "FROM", "TO", "AMOUNT", "REFERENCE", "RECONCILED(from/to)", "ID"}
	case "ManualJournals":
		header = []string{"DATE", "STATUS", "NARRATION", "DEBITS", "CASH-BASIS", "ID"}
	case "Contacts":
		header = []string{"NAME", "EMAIL", "CUSTOMER", "SUPPLIER", "STATUS", "ID"}
	}
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "Organisation: %s (%s)\n", cellSanitizer.Replace(org.Name), cellSanitizer.Replace(org.ID))
	if err := Table(&buffer, header, rows, bufferedColor(w, color)); err != nil {
		return err
	}
	_, err := w.Write(buffer.Bytes())
	return err
}

func bufferedColor(w io.Writer, color string) string {
	if ColorEnabled(w, color) {
		return "always"
	}
	return "never"
}

// Record renders saved document details with exact line amounts and native tracking names.
func Record(w io.Writer, resource string, record xero.Record, attachments, color string) error {
	var fields []string
	switch resource {
	case "BankTransactions":
		fields = []string{"BankTransactionID", "Type", "Status", "Date", "Contact", "BankAccount", "Reference", "LineAmountTypes", "SubTotal", "TotalTax", "Total", "CurrencyCode", "IsReconciled", "HasAttachments", "UpdatedDateUTC"}
	case "ManualJournals":
		fields = []string{"ManualJournalID", "Status", "Date", "Narration", "LineAmountTypes", "ShowOnCashBasisReports", "Url", "HasAttachments", "UpdatedDateUTC"}
	case "BankTransfers":
		fields = []string{"BankTransferID", "Status", "Date", "FromBankAccount", "ToBankAccount", "Amount", "Reference", "CurrencyRate", "FromBankTransactionID", "ToBankTransactionID", "FromIsReconciled", "ToIsReconciled", "FromTracking", "ToTracking", "HasAttachments", "CreatedDateUTC"}
	case "Contacts":
		fields = record.ScalarKeys()
	}
	var rows [][]string
	for _, key := range fields {
		value := record.Text(key)
		switch key {
		case "Contact":
			value = nameWithID(record.Object(key).Text("Name"), record.Object(key).Text("ContactID"))
		case "BankAccount", "FromBankAccount", "ToBankAccount":
			value = nameWithID(accountLabel(record.Object(key)), record.Object(key).Text("AccountID"))
		case "HasAttachments":
			value = yesNo(attachments)
		case "IsReconciled", "FromIsReconciled", "ToIsReconciled", "ShowOnCashBasisReports", "IsCustomer", "IsSupplier":
			value = yesNo(value)
		case "FromTracking", "ToTracking":
			value = trackingLabel(record.Records(key))
		}
		rows = append(rows, []string{key, value})
	}
	var buffer bytes.Buffer
	color = bufferedColor(w, color)
	if err := Table(&buffer, nil, rows, color); err != nil {
		return err
	}
	if resource == "BankTransactions" || resource == "ManualJournals" {
		key := "LineItems"
		header := []string{"ACCOUNT", "DESCRIPTION", "QTY", "UNIT", "TAX TYPE", "TAX", "AMOUNT", "TRACKING"}
		if resource == "ManualJournals" {
			key = "JournalLines"
			header = []string{"ACCOUNT", "DESCRIPTION", "DEBIT", "CREDIT", "TAX TYPE", "TAX", "TRACKING"}
		}
		rows = nil
		for _, line := range record.Records(key) {
			account := line.Text("AccountCode")
			if account == "" {
				account = line.Text("AccountID")
			}
			if resource == "BankTransactions" {
				rows = append(rows, []string{account, line.Text("Description"), line.Text("Quantity"), line.Text("UnitAmount"), line.Text("TaxType"), line.Text("TaxAmount"), line.Text("LineAmount"), trackingLabel(line.Records("Tracking"))})
			} else {
				debit, credit := line.Text("LineAmount"), ""
				if strings.HasPrefix(debit, "-") {
					credit, debit = strings.TrimPrefix(debit, "-"), ""
				}
				rows = append(rows, []string{account, line.Text("Description"), debit, credit, line.Text("TaxType"), line.Text("TaxAmount"), trackingLabel(line.Records("Tracking"))})
			}
		}
		buffer.WriteByte('\n')
		if err := Table(&buffer, header, rows, color); err != nil {
			return err
		}
	}
	if resource == "Contacts" {
		if err := contactDetails(&buffer, record, color); err != nil {
			return err
		}
	}
	_, err := w.Write(buffer.Bytes())
	return err
}

func contactDetails(w io.Writer, record xero.Record, color string) error {
	var rows [][]string
	for _, phone := range record.Records("Phones") {
		rows = append(rows, []string{phone.Text("PhoneType"), phone.Text("PhoneCountryCode"), phone.Text("PhoneAreaCode"), phone.Text("PhoneNumber")})
	}
	if err := Table(w, []string{"PHONE", "COUNTRY", "AREA", "NUMBER"}, rows, color); err != nil {
		return err
	}
	rows = nil
	for _, address := range record.Records("Addresses") {
		var parts []string
		for _, key := range []string{"AttentionTo", "AddressLine1", "AddressLine2", "AddressLine3", "AddressLine4", "City", "Region", "PostalCode", "Country"} {
			if value := address.Text(key); value != "" {
				parts = append(parts, value)
			}
		}
		rows = append(rows, []string{address.Text("AddressType"), strings.Join(parts, ", ")})
	}
	if err := Table(w, []string{"ADDRESS", "DETAILS"}, rows, color); err != nil {
		return err
	}
	rows = nil
	for _, side := range []struct{ label, key string }{{"Receivable", "AccountsReceivable"}, {"Payable", "AccountsPayable"}} {
		balance := record.Object("Balances").Object(side.key)
		rows = append(rows, []string{side.label, balance.Text("Outstanding"), balance.Text("Overdue")})
	}
	if err := Table(w, []string{"BALANCE", "OUTSTANDING", "OVERDUE"}, rows, color); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\nDefaults applied by Xero to new transactions for this contact:"); err != nil {
		return err
	}
	return Table(w, nil, [][]string{
		{"Sales tax type", record.Text("AccountsReceivableTaxType")},
		{"Purchase tax type", record.Text("AccountsPayableTaxType")},
		{"Sales tracking", trackingLabel(record.Records("SalesTrackingCategories"))},
		{"Purchase tracking", trackingLabel(record.Records("PurchasesTrackingCategories"))},
	}, color)
}
