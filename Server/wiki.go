package main

import (
	"fmt"
	"time"
	"io/ioutil"
	"encoding/json"
	"net/http"
	"appengine"
	"appengine/datastore"
)

type Page struct {
	Key	string
	Title	string
	Body	string
	Date	time.Time
}

type Transaction struct {
	Key	string
	Type	string
	Page	Page
	Date	time.Time
}

type HttpStatus struct {
	Status			string
	ErrorMessage	string
	Page				Page
}

func init() {
	http.HandleFunc("/get/", getHandler)
	http.HandleFunc("/get/recent", getRecentTransactionsHandler)
	http.HandleFunc("/post/", postHandler)
	http.HandleFunc("/delete/", deleteHandler)
	http.HandleFunc("/reset", resetHandler)
}

func getHandler(writer http.ResponseWriter, request *http.Request) {
	context := appengine.NewContext(request)
	keyString := request.URL.Path[len("/get/"):]
	key,_ := datastore.DecodeKey(keyString)

	if keyString == "" {
		returnPages(context, writer)
	} else {
		returnPage(context, writer, key)
	}
}

func getRecentTransactionsHandler(writer http.ResponseWriter, request *http.Request) {
	context := appengine.NewContext(request)
	transactionKeyString := request.FormValue("transactionKey")
	sinceString := request.FormValue("sinceDate")
	var sinceDate time.Time
	var err error
	if sinceString != "" {
		//I would like to have used one of the formats provided by the time library.
		//But many of them required the user to compute the day of the week, or were
		//not specific enough, or are currently broken in Go (I'm looking at you RFC3339)
		sinceDate, err = time.Parse("02-01-2006-15:04:05-0700", sinceString)
		if err != nil {
			returnError(writer, err)
		}
	}
	returnTransactions(context, writer, transactionKeyString, sinceDate)
}

func postHandler(writer http.ResponseWriter, request *http.Request) {
	context := appengine.NewContext(request)
	page := getPageDataFromRequest(request)
	err, _ := page.save(context)
	if err != nil {
		returnError(writer, err)
	} else {
		httpStatus := &HttpStatus{Status: "success", Page: *page}
		jsonHttpStatus, _ := json.Marshal(httpStatus)
		fmt.Fprintf(writer, string(jsonHttpStatus))
	}
}

func deleteHandler(writer http.ResponseWriter, request *http.Request) {
	context := appengine.NewContext(request)
	keyString := request.URL.Path[len("/delete/"):]
	if key,err := datastore.DecodeKey(keyString); err != nil {
		returnError(writer, err)
	} else {
		fn := func(context appengine.Context) error {
			var err error
			if err := datastore.Delete(context, key); err == nil {
				transaction := Transaction{Type: "delete", Page: Page{Key:key.Encode()}, Date: time.Now()}
				transKey := datastore.NewIncompleteKey(context, "Transaction", nil)
				if _ , err := datastore.Put(context, transKey, &transaction); err == nil {
					return nil
				} else {
					panic(fmt.Sprintf(err.Error()))
				}
			}
			return err
		}

		err := datastore.RunInTransaction(context, fn, &datastore.TransactionOptions{XG: true})
		if err != nil {
			returnError(writer, err)
		} else {
			page := Page{Key: key.Encode()}
			httpStatus := &HttpStatus{Status: "success", Page: page}
			jsonHttpStatus, _ := json.Marshal(httpStatus)
			fmt.Fprintf(writer, string(jsonHttpStatus))
		}
	}
}

func resetHandler(writer http.ResponseWriter, request *http.Request) {
	context := appengine.NewContext(request)

	query := datastore.NewQuery("Page").KeysOnly()
	if keys, err := query.GetAll(context, nil); err != nil {
		returnError(writer, err)
	} else if err := datastore.DeleteMulti(context, keys); err != nil {
		returnError(writer, err)
	}

	query = datastore.NewQuery("Transaction").KeysOnly()
	if keys, err := query.GetAll(context, nil); err != nil {
		returnError(writer, err)
	} else if err := datastore.DeleteMulti(context, keys); err != nil {
		returnError(writer, err)
	}
	httpStatus := &HttpStatus{Status: "success"}
	jsonHttpStatus, _ := json.Marshal(httpStatus)
	fmt.Fprintf(writer, string(jsonHttpStatus))
}

func getPageDataFromRequest(request *http.Request) (*Page) {
	jsonBody,_ := ioutil.ReadAll(request.Body)
	var page Page
	_ = json.Unmarshal(jsonBody, &page)
	page.Date = time.Now()
	if page.Key == "" {
		keyString := request.URL.Path[len("/post/"):]
		if keyString != "" {
			page.Key = keyString
		}
	}
	return &page
}

func (page *Page) save(context appengine.Context) (error, string) {
	var pageKey *datastore.Key
	var transType string
	if page.Key != "" {
		transType = "update"
		pageKey,_ = datastore.DecodeKey(page.Key)
	} else {
		transType = "insert"
		pageKey = datastore.NewIncompleteKey(context, "Page", nil)
	}

	fn := func(context appengine.Context) error {
		var err error
		if pageKey, err := datastore.Put(context, pageKey, page); err == nil {
			page.Key = pageKey.Encode()
			if pageKey, err := datastore.Put(context, pageKey, page); err == nil {
				transaction := Transaction{Type: transType, Page: *page, Date: page.Date}
				transKey := datastore.NewIncompleteKey(context, "Transaction", nil)
				if _ , err := datastore.Put(context, transKey, &transaction); err == nil {
					page.Key = pageKey.Encode()
					return nil
				} else {
					panic(fmt.Sprintf(err.Error()))
				}
			}
		}
		return err
	}

	err := datastore.RunInTransaction(context, fn, &datastore.TransactionOptions{XG: true})

	return err, page.Key
}

func loadPage(context appengine.Context, key *datastore.Key) (Page, error) {
	var page Page
	err := datastore.Get(context, key, &page)
	return page, err
}

func loadPages(context appengine.Context) ([]Page, []*datastore.Key, error) {
	query := datastore.NewQuery("Page")
	var pages []Page
	keys, err := query.GetAll(context, &pages)
	return pages, keys, err
}

func loadTransactions(context appengine.Context, since time.Time) ([]Transaction, []*datastore.Key, error) {
	query := datastore.NewQuery("Transaction").Filter("Date >", since).Order("Date")
	var transactions []Transaction
	keys, err := query.GetAll(context, &transactions)
	return transactions, keys, err
}

func returnPage(context appengine.Context, writer http.ResponseWriter, key *datastore.Key) {
	if page, err := loadPage(context, key); err != nil {
		returnError(writer, err)
	} else {
		page.Key = key.Encode()
		jsonPage,_ := json.Marshal(page)
		writer.Header().Set("content-type", "application/json")
		fmt.Fprintf(writer, string(jsonPage))
	}
}

func returnPages(context appengine.Context, writer http.ResponseWriter) {
	if pages, keys, err := loadPages(context); err != nil {
		returnError(writer, err)
	} else {
		updatedPages := make([]Page, len(pages), len(pages))
		for index, page := range pages {
			key := keys[index]
			page.Key = key.Encode()
			updatedPages[index] = page
		}
		jsonPage,_ := json.Marshal(updatedPages)
		writer.Header().Set("content-type", "application/json")
		fmt.Fprintf(writer, string(jsonPage))
	}
}

func returnTransactions(context appengine.Context, writer http.ResponseWriter, transactionKeyString string, sinceDate time.Time) {
	var transactions []Transaction
	var keys []*datastore.Key
	var err error
	if sinceDate.IsZero() && transactionKeyString == "" {
		// loading transactions with a sinceDate equal to zero will return *all* transactions
		transactions, keys, err = loadTransactions(context, sinceDate)
	} else if sinceDate.IsZero() {
		//Get date from transaction. Use sinceDate for query
		previousTransactionKey,err := datastore.DecodeKey(transactionKeyString)
		if err != nil {
			returnError(writer,err)
		}

		var previousTransaction Transaction
		err = datastore.Get(context, previousTransactionKey, &previousTransaction)
		if err != nil {
			returnError(writer, err)
		}
		sinceDate = previousTransaction.Date
		transactions, keys, err = loadTransactions(context, sinceDate)
	} else {
		// Use sinceDate for query
		transactions, keys, err = loadTransactions(context, sinceDate)
	}

	if err != nil {
		returnError(writer, err)
	}

	updatedTransactions := make([]Transaction, len(transactions), len(transactions))
	for index, transaction := range transactions {
		key := keys[index]
		transaction.Key = key.Encode()
		updatedTransactions[index] = transaction
	}
	jsonTransaction,_ := json.Marshal(updatedTransactions)
	writer.Header().Set("content-type", "application/json")
	fmt.Fprintf(writer, string(jsonTransaction))
}

func returnError(writer http.ResponseWriter, err error) {
		httpStatus := &HttpStatus{Status: "error", ErrorMessage: err.Error()}
		jsonHttpStatus, _ := json.Marshal(httpStatus)
		http.Error(writer, string(jsonHttpStatus), 500)
}
